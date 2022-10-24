package main

import (
	"context"
	"flag"
	"fmt"
	"github.com/aaronland/go-json-query"
	jw "github.com/aaronland/go-jsonl/walk"
	"github.com/aaronland/go-libraryofcongress-datajam"
	"github.com/aaronland/go-libraryofcongress-datajam/walk"
	"github.com/sfomuseum/go-csvdict"
	"github.com/tidwall/gjson"
	_ "gocloud.dev/blob/fileblob"
	_ "gocloud.dev/blob/s3blob"
	"io"
	"io/ioutil"
	"log"
	"os"
	"strings"
	"sync"
)

func main() {

	bucket_uri := flag.String("bucket-uri", "", "A valid GoCloud bucket URI. Valid schemes are: file://, s3:// and loc:// which is signals that data should be retrieved from the Library Of Congress's '...' S3 bucket.")
	workers := flag.Int("workers", 10, "The maximum number of concurrent workers. This is used to prevent filehandle exhaustion.")

	to_stdout := flag.Bool("stdout", true, "Emit to STDOUT")
	to_devnull := flag.Bool("null", false, "Emit to /dev/null")

	var queries query.QueryFlags
	flag.Var(&queries, "query", "One or more {PATH}={REGEXP} parameters for filtering records.")

	valid_modes := strings.Join([]string{query.QUERYSET_MODE_ALL, query.QUERYSET_MODE_ANY}, ", ")
	desc_modes := fmt.Sprintf("Specify how query filtering should be evaluated. Valid modes are: %s", valid_modes)

	query_mode := flag.String("query-mode", query.QUERYSET_MODE_ALL, desc_modes)

	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage:\n")
		fmt.Fprintf(os.Stderr, "  %s [options] [path1 path2 ... pathN]\n\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "Options:\n")
		flag.PrintDefaults()
	}

	flag.Parse()

	ctx := context.Background()

	ctx, bucket, err := datajam.OpenBucket(ctx, *bucket_uri)

	if err != nil {
		log.Fatalf("Failed to open bucket, %v", err)
	}

	defer bucket.Close()

	writers := make([]io.Writer, 0)

	if *to_stdout {
		writers = append(writers, os.Stdout)
	}

	if *to_devnull {
		writers = append(writers, ioutil.Discard)
	}

	if len(writers) == 0 {
		log.Fatal("Nothing to write to.")
	}

	wr := io.MultiWriter(writers...)

	fieldnames := []string{
		"id",
		"location",
		"wof_id",
	}

	csv_wr, err := csvdict.NewWriter(wr, fieldnames)

	if err != nil {
		log.Fatalf("Failed to create CSV writer, %v", err)
	}

	csv_wr.WriteHeader()

	// location is an array of names
	// locations is an array of empty GeoJSON features
	queries.Set("location=.*")

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	mu := new(sync.RWMutex)

	cb := func(ctx context.Context, rec *jw.WalkRecord, err error) error {

		if err != nil {

			if jw.IsEOFError(err) {
				return nil
			}

			log.Println(err)
			return err
		}

		id_rsp := gjson.GetBytes(rec.Body, "item.id")
		id := id_rsp.String()

		// At least with the sample set anything with a 'location' also has a 'latlong'

		/*
			coords_rsp := gjson.GetBytes(rec.Body, "latlong")

			if coords_rsp.Exists() {
				fmt.Println("NO")
				return nil
			}
		*/

		loc_rsp := gjson.GetBytes(rec.Body, "location")
		locs := loc_rsp.Array()

		locations := make([]string, len(locs))

		for idx, r := range loc_rsp.Array() {
			locations[idx] = r.String()
		}

		// This works for most of the cases but it is understood that some records
		// have been encoded incorrectly

		for i, j := 0, len(locations)-1; i < j; i, j = i+1, j-1 {
			locations[i], locations[j] = locations[j], locations[i]
		}

		str_loc := strings.Join(locations, ",")

		out := map[string]string{
			"id":       id,
			"location": str_loc,
			"wof_id":   "-1",
		}

		mu.Lock()
		defer mu.Unlock()

		err = csv_wr.WriteRow(out)

		if err != nil {
			return fmt.Errorf("Failed to write row for %s, %w", id, err)
		}

		return nil
	}

	uris := flag.Args()

	filter_func := func(ctx context.Context, uri string) bool {
		// Skip things like index.txt' or errant 'fileblob*' records
		return true
	}

	for _, uri := range uris {

		opts := &walk.WalkOptions{
			URI:      uri,
			Workers:  *workers,
			Callback: cb,
			IsBzip:   false,
			Filter:   filter_func,
		}

		if len(queries) > 0 {

			qs := &query.QuerySet{
				Queries: queries,
				Mode:    *query_mode,
			}

			opts.QuerySet = qs
		}

		err := walk.WalkBucket(ctx, opts, bucket)

		if err != nil {
			log.Fatalf("Failed to crawl %s, %v", uri, err)
		}
	}

	csv_wr.Flush()
}

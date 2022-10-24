package main

import (
	"bytes"
	"context"
	_ "encoding/json"
	"flag"
	"fmt"
	"github.com/aaronland/go-json-query"
	jw "github.com/aaronland/go-jsonl/walk"
	"github.com/aaronland/go-libraryofcongress-datajam"
	"github.com/aaronland/go-libraryofcongress-datajam/walk"
	_ "gocloud.dev/blob/fileblob"
	_ "gocloud.dev/blob/s3blob"
	"io"
	"io/ioutil"
	"log"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

func main() {

	bucket_uri := flag.String("bucket-uri", "", "A valid GoCloud bucket URI. Valid schemes are: file://, s3:// and loc:// which is signals that data should be retrieved from the Library Of Congress's '...' S3 bucket.")
	workers := flag.Int("workers", 10, "The maximum number of concurrent workers. This is used to prevent filehandle exhaustion.")

	to_stdout := flag.Bool("stdout", true, "Emit to STDOUT")
	to_devnull := flag.Bool("null", false, "Emit to /dev/null")

	as_json := flag.Bool("json", false, "Emit a JSON list.")
	validate_json := flag.Bool("validate-json", false, "Ensure each record is valid JSON.")
	format_json := flag.Bool("format-json", false, "Format JSON output for each record.")

	// as_oembed := flag.Bool("oembed", false, "Emit results as OEmbed records")

	stats := flag.Bool("stats", false, "Display timings and statistics.")

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

	count := uint32(0)

	if *stats {

		t1 := time.Now()

		defer func() {

			final_count := atomic.LoadUint32(&count)
			log.Printf("Processed %d records in %v\n", final_count, time.Since(t1))
		}()
	}

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	mu := new(sync.RWMutex)

	write := func(ctx context.Context, records ...[]byte) error {

		mu.Lock()
		defer mu.Unlock()

		for _, body := range records {

			select {
			case <-ctx.Done():
				return nil
			default:
				// pass
			}

			body = bytes.TrimSpace(body)

			new_count := atomic.AddUint32(&count, 1)

			if *as_json && new_count > 1 {
				wr.Write([]byte(","))
			}

			wr.Write(body)
			wr.Write([]byte("\n"))
		}

		return nil
	}

	cb := func(ctx context.Context, rec *jw.WalkRecord, err error) error {

		if err != nil {

			if jw.IsEOFError(err) {
				return nil
			}

			log.Println(err)
			return err
		}

		records := make([][]byte, 0)

		/*
			if *as_oembed {

			var object map[string]interface{}
				err = json.Unmarshal(rec.Body, &object)

				if err != nil {
					log.Println(err)
					return err
				}

				if *as_oembed {

					oembed_records, err := oembed.OEmbedRecordsFromOpenAccessRecord(object)

					if err != nil {
						// log.Printf("Unable to construct oembed records from object '%s': %v\n", object.Id, err)
						return nil
					}

					for _, o_rec := range oembed_records {

						body, err := json.Marshal(o_rec)

						if err != nil {
							return err
						}

						records = append(records, body)
					}

				} else {
					records = append(records, rec.Body)
				}

				records = append(records, rec.Body)

			} else {
				records = append(records, rec.Body)
			}

		*/

		records = append(records, rec.Body)

		return write(ctx, records...)
	}

	uris := flag.Args()

	if *as_json {
		wr.Write([]byte("["))
	}

	filter_func := func(ctx context.Context, uri string) bool {
		// Skip things like index.txt' or errant 'fileblob*' records
		return true
	}

	for _, uri := range uris {

		opts := &walk.WalkOptions{
			URI:          uri,
			Workers:      *workers,
			FormatJSON:   *format_json,
			ValidateJSON: *validate_json,
			Callback:     cb,
			IsBzip:       false,
			Filter:       filter_func,
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

	if *as_json {
		wr.Write([]byte("]"))
	}

}

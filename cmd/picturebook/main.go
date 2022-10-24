package main

import (
	"context"
	"flag"
	"fmt"
	"github.com/aaronland/go-json-query"
	jw "github.com/aaronland/go-jsonl/walk"
	"github.com/aaronland/go-libraryofcongress-datajam"
	"github.com/aaronland/go-libraryofcongress-datajam/walk"
	"github.com/aaronland/go-picturebook"
	"github.com/aaronland/go-picturebook/picture"
	"github.com/tidwall/gjson"
	"gocloud.dev/blob"
	_ "gocloud.dev/blob/fileblob"
	_ "gocloud.dev/blob/memblob"
	_ "gocloud.dev/blob/s3blob"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
)

func main() {

	bucket_uri := flag.String("bucket-uri", "", "A valid GoCloud bucket URI. Valid schemes are: file://, s3:// and loc:// which is signals that data should be retrieved from the Library Of Congress's '...' S3 bucket.")
	workers := flag.Int("workers", 10, "The maximum number of concurrent workers. This is used to prevent filehandle exhaustion.")

	var queries query.QueryFlags
	flag.Var(&queries, "query", "One or more {PATH}={REGEXP} parameters for filtering records.")

	valid_modes := strings.Join([]string{query.QUERYSET_MODE_ALL, query.QUERYSET_MODE_ANY}, ", ")
	desc_modes := fmt.Sprintf("Specify how query filtering should be evaluated. Valid modes are: %s", valid_modes)

	query_mode := flag.String("query-mode", query.QUERYSET_MODE_ALL, desc_modes)

	filename := flag.String("filename", "picturebook.pdf", "The (relative) name of the final PDF file.")

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

	cwd, err := os.Getwd()

	if err != nil {
		log.Fatalf("Failed to get working directory, %v", err)
	}

	pb_uri := fmt.Sprintf("file://%s", cwd)
	pb_bucket, err := blob.OpenBucket(ctx, pb_uri)

	if err != nil {
		log.Fatalf("Failed to create picturebook bucket, %v", err)
	}

	defer pb_bucket.Close()

	pagenum := int64(0)

	pb_opts, err := picturebook.NewPictureBookDefaultOptions(ctx)

	if err != nil {
		log.Fatalf("Unable to create picturebook options, %v", err)
	}

	pb_opts.Target = pb_bucket
	// pb_opts.Orientation = "L"
	pb_opts.Width = 9
	pb_opts.Height = 7
	pb_opts.FillPage = true
	pb_opts.Verbose = false

	pb, err := picturebook.NewPictureBook(ctx, pb_opts)

	if err != nil {
		log.Fatalf("Failed to create picturebook, %w", err)
	}

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	cb := func(ctx context.Context, rec *jw.WalkRecord, err error) error {

		if err != nil {

			if jw.IsEOFError(err) {
				return nil
			}

			log.Println(err)
			return err
		}

		if pagenum > 10 {
			// return nil
		}

		title_rsp := gjson.GetBytes(rec.Body, "item.title")
		id_rsp := gjson.GetBytes(rec.Body, "item.id")

		caption := fmt.Sprintf("%s #%s", title_rsp.String(), id_rsp.String())

		url_rsp := gjson.GetBytes(rec.Body, "item.service_medium")
		im_url := url_rsp.String()

		if im_url == "" {
			return nil
		}

		// Is this an S3 bucket? What is it? What is the region?
		// https://tile.loc.gov/storage-services/service/pnp/stereo/1s10000/1s13000/1s13400/1s13435r.jpg

		im_rsp, err := http.Get(im_url)

		if err != nil {
			return fmt.Errorf("Failed to GET %s, %w", im_url, err)
		}

		im_fname := filepath.Base(im_url)

		im_bucket, err := blob.OpenBucket(ctx, "mem://")

		if err != nil {
			return fmt.Errorf("Failed to open image bucket, %w", err)
		}

		defer im_bucket.Close()

		im_wr, err := im_bucket.NewWriter(ctx, im_fname, nil)

		if err != nil {
			return fmt.Errorf("Failed to create new writer, %w", err)
		}

		_, err = io.Copy(im_wr, im_rsp.Body)

		if err != nil {
			return fmt.Errorf("Failed to copy %s to %s, %w", im_url, im_fname, err)
		}

		err = im_wr.Close()

		if err != nil {
			return fmt.Errorf("Failed to close %s, %w", im_fname, err)
		}

		pb_picture := &picture.PictureBookPicture{
			Source:  im_fname,
			Path:    im_fname,
			Bucket:  im_bucket,
			Caption: caption,
		}

		pg := atomic.AddInt64(&pagenum, 1)

		err = pb.AddPicture(ctx, int(pg), pb_picture)

		if err != nil {
			return fmt.Errorf("Failed to add picture for %s, %w", im_url, err)
		}

		log.Printf("Added %s (%s) on page %d\n", caption, im_url, pg)
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

	err = pb.Save(ctx, *filename)

	if err != nil {
		log.Fatalf("Failed to save picturebook, %v", err)
	}
}

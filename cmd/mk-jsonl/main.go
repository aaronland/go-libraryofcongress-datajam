package main

import (
	"encoding/json"
	"flag"
	"log"
	"os"
)

func main() {

	flag.Parse()

	for _, path := range flag.Args() {

		r, err := os.Open(path)

		if err != nil {
			log.Fatalf("Failed to open %s, %w", path, err)
		}

		defer r.Close()

		var data map[string]interface{}

		dec := json.NewDecoder(r)
		err = dec.Decode(&data)

		if err != nil {
			log.Fatalf("Failed to decode data, %w", err)
		}

		for id, details := range data {

			enc := json.NewEncoder(os.Stdout)
			err := enc.Encode(details)

			if err != nil {
				log.Fatalf("Failed to encode ID %s, %w", id, err)
			}
		}
	}
}

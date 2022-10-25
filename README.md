# go-libraryofcongress-datajam

## Documentation

Documentation is incomplete.

## Tools

### mk-jsonl

Convert LoC `metadata.json` data in to line-separated JSON.

```
$> go run cmd/mk-jsonl/main.go \
	../metadata.json \
	> ../metadata.jsonl
```

### emit

Emit one or more records from a line-seperated JSON file (see above), optionally filtering on zero or more properties.

```
$> go run -mod vendor cmd/emit/main.go \
	-query='item.id=2015651792' \
	-bucket-uri file:///path/to/data-folder/ \
	-json \
	-format-json \
	data
```

### picturebook

Create a PDF file containing images derived from one or more records from a line-seperated JSON file (see above), optionally filtering on zero or more properties.

```
$> go run -mod vendor cmd/picturebook/main.go \
	-bucket-uri file:///path/to-data-folder/ \
	-query 'date=1861' \
	data
```

For example:

* [examples/picturebook.pdf](examples/picturebook.pdf)

### featurecollection

Create a GeoJSON file derived from one or more records from a line-seperated JSON file (see above) with location information.

```
$> go run -mod vendor cmd/featurecollection/main.go \
	-bucket-uri file:///path/to/data-folder/ \
	data \
	> loc.geojson
```

For example:

* [examples/loc.geojson](examples/loc.geojson)

### to-geocode

Create a CSV file derived from one or more records from a line-seperated JSON file (see above) with location information to be geocoded (to determine canonical location identifiers).

```
$> go run -mod vendor cmd/to-geocode/main.go \
	-bucket-uri file:///path/to/data-folder/ \
	data
```

For example:

* [examples/to-geocode.csv](examples/to-geocode.csv]
* [examples/loc-geocoded.csv](examples/loc-geocoded.csv]

## See also

* https://github.com/aaronland/go-jsonl
* https://github.com/aaronland/go-json-query
* https://github.com/aaronland/go-picturebook
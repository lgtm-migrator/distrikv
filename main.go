package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"

	"github.com/fffzlfk/distrikv/config"
	"github.com/fffzlfk/distrikv/db"

	"github.com/fffzlfk/distrikv/httpd"
)

var (
	dbLocation     = flag.String("db-location", "", "the path to the bolt db database")
	httpAddr       = flag.String("http-addr", "", "set-addr")
	configFileName = flag.String("config-file", "sharding.toml", "set-config-file")
	shard          = flag.String("shard", "", "select the shard")
)

func init() {
	flag.Parse()
	if *httpAddr == "" {
		log.Fatal("Must provide http-addr")
	}

	if *dbLocation == "" {
		log.Fatal("Must provide db-location")
	}

	if *shard == "" {
		log.Fatal("Must provide shard")
	}
}

func main() {

	cfg, err := config.ParseFile(*configFileName)
	if err != nil {
		log.Fatal(err)
	}

	shards, err := config.ParseShards(cfg.Shards, *shard)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("Shard count = %d, current shard: %d\n", shards.Count, shards.Index)

	db, close, err := db.NewDatabase(*dbLocation)
	if err != nil {
		log.Fatalf("NewDataBase(%q): %v", *dbLocation, err)
	}
	defer close()

	server := httpd.NewServer(db, shards)

	http.HandleFunc("/get", server.GetHandler)

	http.HandleFunc("/set", server.SetHandler)

	// hash(key) % count = <current index>

	log.Fatal(server.ListenAndServe(*httpAddr))
}

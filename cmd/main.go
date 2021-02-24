package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/sunshineplan/capital-flows-data/sector"
	"github.com/sunshineplan/utils/database/mongodb"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
)

var config = mongodb.Config{
	Database:   "stock",
	Collection: "capitalflows",
	Username:   "capitalflows",
	Password:   "capitalflows",
	SRV:        true,
}

var path string
var collection *mongo.Collection

func main() {
	flag.StringVar(&config.Server, "mongo", "", "MongoDB Server")
	flag.StringVar(&path, "path", "", "data path")
	flag.Parse()

	if err := connect(); err != nil {
		log.Fatal(err)
	}

	if err := backup(); err != nil {
		log.Fatal(err)
	}
}

func connect() error {
	client, err := config.Open()
	if err != nil {
		return err
	}

	collection = client.Database(config.Database).Collection(config.Collection)

	return nil
}

func backup() error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	cur, err := collection.Aggregate(ctx, []interface{}{bson.M{"$group": bson.M{"_id": "$date"}}})
	if err != nil {
		return err
	}

	ctx, cancel = context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	var date []struct {
		Date string `bson:"_id"`
	}
	if err := cur.All(ctx, &date); err != nil {
		return err
	}

	tz, _ := time.LoadLocation("Asia/Shanghai")
	t := time.Now().In(tz)
	today := fmt.Sprintf("%d-%02d-%02d", t.Year(), t.Month(), t.Day())

	for _, i := range date {
		if i.Date != today {
			res, err := sector.GetTimeLine(i.Date, collection)
			if err != nil {
				return err
			}

			b, err := json.Marshal(res)
			if err != nil {
				return err
			}

			fullpath := filepath.Join(append([]string{path}, strings.Split(i.Date, "-")...)...)
			if err := os.MkdirAll(filepath.Dir(fullpath), 0744); err != nil {
				return err
			}

			if err := os.WriteFile(fullpath+".json", b, 0744); err != nil {
				return err
			}

			d, _ := time.ParseInLocation("2006-01-02", i.Date, tz)
			if t.Sub(d).Hours() > 7*24 {
				if err := delete(i.Date); err != nil {
					return err
				}
			}
		}
	}

	return nil
}

func delete(date string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	_, err := collection.DeleteMany(ctx, bson.M{"date": date})

	return err
}

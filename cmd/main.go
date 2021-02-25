package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/go-github/github"
	"github.com/sunshineplan/capital-flows-data/sector"
	"github.com/sunshineplan/utils/database/mongodb"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"golang.org/x/oauth2"
)

var config = mongodb.Config{
	Database:   "stock",
	Collection: "capitalflows",
	Username:   "capitalflows",
	Password:   "capitalflows",
	SRV:        true,
}

var token, repository string
var path string
var collection *mongo.Collection

func main() {
	flag.StringVar(&config.Server, "mongo", "", "MongoDB Server")
	flag.StringVar(&token, "token", "", "token")
	flag.StringVar(&repository, "repo", "", "repository")
	flag.StringVar(&path, "path", "", "data path")
	flag.Parse()

	if err := connect(); err != nil {
		log.Fatal(err)
	}

	if err := commit(); err != nil {
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

func commit() error {
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

	ts := oauth2.StaticTokenSource(&oauth2.Token{AccessToken: token})

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

			tc := oauth2.NewClient(ctx, ts)
			client := github.NewClient(tc)
			repo := strings.Split(repository, "/")
			fullpath := filepath.Join(append([]string{path}, strings.Split(i.Date, "-")...)...) + ".json"
			opt := &github.RepositoryContentFileOptions{
				Message: github.String(i.Date),
				Content: b,
			}

			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()

			if _, _, err := client.Repositories.CreateFile(ctx, repo[0], repo[1], fullpath, opt); err != nil {
				if !strings.Contains(err.Error(), `"sha" wasn't supplied.`) {
					return err
				}
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

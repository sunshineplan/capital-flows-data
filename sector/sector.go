package sector

import (
	"context"
	"time"

	"github.com/sunshineplan/utils/database/mongodb"
	"go.mongodb.org/mongo-driver/bson"
)

// Chart contains one day chart data.
type Chart struct {
	Sector string `json:"sector" bson:"_id"`
	Chart  []struct {
		X string `json:"x"`
		Y int64  `json:"y"`
	} `json:"chart"`
}

// Query all sectors chart data of one day.
func Query(mongo mongodb.Config, date string) ([]Chart, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	client, err := mongo.Open()
	if err != nil {
		return nil, err
	}
	defer client.Disconnect(ctx)

	collection := client.Database(mongo.Database).Collection(mongo.Collection)

	var pipeline []interface{}
	pipeline = append(pipeline, bson.M{"$match": bson.M{"date": date}})
	pipeline = append(pipeline, bson.M{"$project": bson.M{"time": 1, "flows": bson.M{"$objectToArray": "$flows"}}})
	pipeline = append(pipeline, bson.M{"$unwind": "$flows"})
	pipeline = append(pipeline,
		bson.M{
			"$group": bson.D{
				bson.E{Key: "_id", Value: "$flows.k"},
				bson.E{Key: "chart", Value: bson.M{"$push": bson.D{
					bson.E{Key: "x", Value: "$time"},
					bson.E{Key: "y", Value: "$flows.v"},
				}}},
			},
		},
	)
	pipeline = append(pipeline, bson.M{"$sort": bson.M{"_id": 1}})

	cur, err := collection.Aggregate(ctx, pipeline)
	if err != nil {
		return nil, err
	}
	defer cur.Close(ctx)

	var res []Chart
	if err := cur.All(ctx, &res); err != nil {
		return nil, err
	}

	return res, nil
}

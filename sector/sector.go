package sector

import (
	"context"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
)

// TimeLine contains one day timeline data.
type TimeLine struct {
	Sector   string `json:"sector" bson:"_id"`
	TimeLine []map[string]int64
}

// Chart contains one day chart data.
type Chart struct {
	Sector string `json:"sector" bson:"_id"`
	Chart  []struct {
		X string `json:"x"`
		Y int64  `json:"y"`
	} `json:"chart"`
}

func query(date string, xy bool, collection *mongo.Collection) (interface{}, error) {
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

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	cur, err := collection.Aggregate(ctx, pipeline)
	if err != nil {
		return nil, err
	}

	ctx, cancel = context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	var charts []Chart
	if err := cur.All(ctx, &charts); err != nil {
		return nil, err
	}

	if xy {
		return charts, nil
	}

	var res []TimeLine
	for _, i := range charts {
		res = append(res, Chart2TimeLine(i))
	}

	return res, nil
}

// GetTimeLine gets all sectors timeline data of one day.
func GetTimeLine(date string, collection *mongo.Collection) ([]TimeLine, error) {
	res, err := query(date, false, collection)
	if err != nil {
		return nil, err
	}

	return res.([]TimeLine), nil
}

// GetChart gets all sectors chart data of one day.
func GetChart(date string, collection *mongo.Collection) ([]Chart, error) {
	res, err := query(date, true, collection)
	if err != nil {
		return nil, err
	}

	return res.([]Chart), nil
}

// Chart2TimeLine convert Chart to TimeLine.
func Chart2TimeLine(chart Chart) TimeLine {
	var data []map[string]int64
	for _, point := range chart.Chart {
		data = append(data, map[string]int64{point.X: point.Y})
	}

	return TimeLine{Sector: chart.Sector, TimeLine: data}
}

// TimeLine2Chart convert TimeLine to Chart.
func TimeLine2Chart(timeline TimeLine) Chart {
	var data []struct {
		X string `json:"x"`
		Y int64  `json:"y"`
	}
	for _, i := range timeline.TimeLine {
		for k, v := range i {
			data = append(data, struct {
				X string `json:"x"`
				Y int64  `json:"y"`
			}{X: k, Y: v})
		}
	}

	return Chart{Sector: timeline.Sector, Chart: data}
}

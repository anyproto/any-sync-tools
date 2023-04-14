package main

import (
	"fmt"
	"github.com/anytypeio/any-sync/nodeconf"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"gopkg.in/yaml.v3"
	"log"
	"os"
	"strings"
	"time"
)

type ConfModel struct {
	Id           primitive.ObjectID `bson:"_id"`
	NetworkId    string             `bson:"networkId"`
	Nodes        []nodeconf.Node    `bson:"nodes"`
	CreationTime time.Time          `bson:"creationTime"`
	Enable       bool               `bson:"enable"`
}

func main() {
	var conf nodeconf.Configuration
	if err := yaml.NewDecoder(os.Stdin).Decode(&conf); err != nil {
		log.Fatalln("can't decode yaml:", err)
	}
	id, err := primitive.ObjectIDFromHex(conf.Id)
	if err != nil {
		log.Fatalln("invalid configurationId:", err)
	}
	cm := ConfModel{
		Id:           id,
		NetworkId:    conf.NetworkId,
		Nodes:        conf.Nodes,
		CreationTime: conf.CreationTime,
		Enable:       false,
	}
	bsonBytes, err := bson.Marshal(cm)
	if err != nil {
		log.Fatalln(err)
	}
	var res bson.D
	if err = bson.Unmarshal(bsonBytes, &res); err != nil {
		log.Fatalln(err)
	}
	js, _ := bson.MarshalExtJSONIndent(cm, false, false, "", "    ")
	old := fmt.Sprintf(`{
        "$oid": "%s"
    }`, cm.Id.Hex())
	jss := strings.Replace(string(js), old, fmt.Sprintf(`ObjectId("%s")`, cm.Id.Hex()), 1)

	const RFC3339Millis = "2006-01-02T15:04:05.000Z07:00"
	old = fmt.Sprintf(`{
        "$date": "%s"
    }`, cm.CreationTime.UTC().Format(RFC3339Millis))
	jss = strings.Replace(jss, old, fmt.Sprintf(`ISODate("%s")`, cm.CreationTime.UTC().Format(RFC3339Millis)), 1)
	fmt.Println(jss)
}

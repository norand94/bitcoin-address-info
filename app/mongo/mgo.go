package mongo

import (
	"github.com/norand94/bitcoin-address-info/app/config"
	"github.com/globalsign/mgo"
)

type Client struct {
	Session *mgo.Session
	DbName string
}

func New(conf config.M) *Client  {
	cli := new(Client)
	cli.DbName = conf.MongoDbName

	var err error
	cli.Session, err = mgo.Dial(conf.MongoDialUrl)
	if err != nil {
		panic(err)
	}

	return cli
}

func (c *Client) NewSession() *mgo.Session  {
	return c.Session.Copy()
}

func (c *Client) Exec(collection string , f func(c *mgo.Collection) error) error {
	s := c.Session.Copy()
	defer s.Close()
	coll := s.DB(c.DbName).C(collection)
	return f(coll)
}


package mongo

import (
	"github.com/globalsign/mgo"
	"github.com/norand94/bitcoin-address-info/app/config"
)

const Address = "address"
const Blocks = "blocks"

type Client struct {
	Session *mgo.Session
	DbName  string
}

func New(conf *config.M) *Client {
	cli := new(Client)
	cli.DbName = conf.Mongo.DbName

	var err error
	cli.Session, err = mgo.Dial(conf.Mongo.DialUrl)
	if err != nil {
		panic(err)
	}

	cli.Session.DB(conf.Mongo.DbName).C(Blocks).EnsureIndex(mgo.Index{
		Key: []string{
			"blocks.address",
		},
	})

	return cli
}

func (c *Client) NewSession() *mgo.Session {
	return c.Session.Copy()
}

func (c *Client) FindOne(collection string, query interface{}, result interface{}) error {
	s := c.Session.Copy()
	defer s.Close()
	return s.DB(c.DbName).C(collection).Find(query).One(result)
}

func (c *Client) Insert(collection string, doc interface{}) error {
	return c.Exec(collection, func(col *mgo.Collection) error {
		return col.Insert(doc)
	})
}

func (c *Client) Exec(collection string, f func(c *mgo.Collection) error) error {
	s := c.Session.Copy()
	defer s.Close()
	return f(s.DB(c.DbName).C(collection))
}

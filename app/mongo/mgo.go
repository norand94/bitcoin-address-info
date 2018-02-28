package mongo

import (
	"github.com/norand94/bitcoin-address-info/app/config"
	"github.com/globalsign/mgo"
)

const AddressInfo = "addressInfo"
const ResponseCache = "responseCache"

type Client struct {
	Session *mgo.Session
	DbName string
}

func New(conf *config.M) *Client  {
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
	return f(s.DB(c.DbName).C(collection))
}

func (c *Client) Find(collection string, query interface{}, result interface{}) error {
	s := c.Session.Copy()
	defer s.Close()
	return s.DB(c.DbName).C(collection).Find(query).All(result)
}


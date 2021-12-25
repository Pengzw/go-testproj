package service

import (
	"net/http"
	"goships/internal/appserver/data"
	"github.com/google/wire"
)

// ProviderSet is service providers.
var (
	ProviderSet = wire.NewSet(NewService)
	HttpOk 		= http.StatusOK
)

type Service struct {
	Data 			*data.Data
}

func NewService(d *data.Data) *Service {
	return &Service{
		Data:  d,
	}
}

type Response struct {
	Code 			int `json:"code"`
	Error 			string `json:"error"`
	Result 			interface{} `json:"result"`
}

func InitResponse() (resp *Response) {
	resp = &Response{
		Code: 200,
	}
	return
}
package client

import (
	"bytes"
	"context"
	"encoding/json"
	"io/ioutil"
	"net/http"
	"strings"

	"golang.org/x/net/context/ctxhttp"
)

// getRequestObject creates JSON-RPC request object.
func getRequestObject(method string, params json.RawMessage) *RequestObject {
	return &RequestObject{
		Jsonrpc: "2.0",
		Method:  method,
		Params:  params,
		ID:      genUUID(),
	}
}

// Call wraps JSON-RPC client call.
func (c *Config) Call(method string, params json.RawMessage) (json.RawMessage, error) {
	var rerr, err error

	// prepare request object
	reqObj := getRequestObject(method, params)

	// convert request object to bytes
	reqData, err := json.Marshal(reqObj)
	if err != nil {
		return nil, NewInternalError(ErrorPrefix, err)
	}

	// prepare request data buffer
	buf := bytes.NewBuffer(reqData)

	// set request type to POST
	req, err := http.NewRequest("POST", c.uri, buf)
	if err != nil {
		return nil, NewInternalError(ErrorPrefix, err)
	}

	// setting specified headers
	for k, v := range c.headers {
		req.Header.Set(k, v)
	}

	// set compression header
	if !c.disableCompression {
		req.Header.Set("Content-Encoding", "gzip")
	}

	// add X-Real-IP, X-Client-IP, when using unix sockets mode
	if c.socketPath != nil {
		req.Header.Set("X-Real-IP", "127.0.0.1")
		req.Header.Set("X-Client-IP", "127.0.0.1")
	}

	// prepare response
	var resp *http.Response

	// set timeout
	ctx, cancel := context.WithTimeout(context.Background(), c.timeout)
	defer cancel()

	// send request
	resp, err = ctxhttp.Do(ctx, c.httpClient, req)
	if err != nil {
		return nil, NewInternalError(ErrorPrefix, err)
	}

	// close response body
	defer resp.Body.Close()

	// fail when HTTP status code is different from 200
	if resp.StatusCode != http.StatusOK {
		return nil, NewInternalError(ErrorPrefix, nil).SetHTTPStatusCodes(resp.StatusCode, http.StatusOK)
	}

	// read response raw bytes data
	respData, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, NewInternalError(ErrorPrefix, err)
	}

	// prepare response object
	respObj := new(ResponseObject)

	// convert response data to object
	err = json.Unmarshal(respData, respObj)
	if err != nil {
		return nil, NewInternalError(ErrorPrefix, err)
	}

	// validate request/response IDs
	if !strings.EqualFold(reqObj.ID, respObj.ID) {
		return nil, NewInternalError(ErrorPrefix, nil).SetRPCIDs(respObj.ID, reqObj.ID)
	}

	// validate request/response Jsonrpc protocol versions
	if !strings.EqualFold(reqObj.Jsonrpc, respObj.Jsonrpc) {
		return nil, NewInternalError(ErrorPrefix, nil).SetProtocolVersions(respObj.Jsonrpc, reqObj.Jsonrpc)
	}

	// check response error
	if respObj.Error != nil {
		return nil, respObj.Error
	}

	// return response result and function-global error
	return respObj.Result, rerr
}

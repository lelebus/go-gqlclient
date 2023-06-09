// Package gqlclient provides a low level GraphQL client.
//
//	// create a client (safe to share across requests)
//	client := gqlclient.NewClient("http://localhost:4000/graphql")
//
//	// to specify your own http.Client, use the WithHTTPClient option:
//	customHttpClient := &http.Client{}
//	customClient := gqlclient.NewClient("http://localhost:4000/graphql", gqlclient.WithHTTPClient(customHttpClient))
//
//	// make a request
//	req := gqlclient.NewRequest(`
//	    query {
//	        items {
//	            field1
//	            field2
//	            field3
//	        }
//	    }
//	`)
//
//	// make a request with variables
//	req := gqlclient.NewRequest(`
//	    query ($key: String!) {
//	        items (id:$key) {
//	            field1
//	            field2
//	            field3
//	        }
//	    }
//	`).WithVars(map[string]interface{}{
//	    "key": "value",
//	})
//
//	// run it and capture the response
//	var responseData map[string]interface{}
//	res, err := client.Run(ctx, req, &responseData)
//	if err != nil {
//	    log.Fatal(err)
//	}
//
//	// read the cookies in the response
//	cookies := res.Cookies()
package gqlclient

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"

	"github.com/pkg/errors"
)

// Client is a client for interacting with a GraphQL API.
type Client struct {
	endpoint         string
	useMultipartForm bool
	httpClient       *http.Client

	// closeReq will close the request body immediately allowing for reuse of client
	closeReq bool

	// Log is called with various debug information.
	// To log to standard out, use:
	//  client.Log = func(s string) { log.Println(s) }
	Log func(s string)
}

// NewClient makes a new Client, optimized for GraphQL requests.
func NewClient(endpoint string, opts ...ClientOption) *Client {
	c := &Client{
		endpoint: endpoint,
		Log:      func(string) {},
	}
	for _, optionFunc := range opts {
		optionFunc(c)
	}
	if c.httpClient == nil {
		c.httpClient = http.DefaultClient
	}
	return c
}

func (c *Client) logf(format string, args ...interface{}) {
	c.Log(fmt.Sprintf(format, args...))
}

// Run executes the query and unmarshals the response from the data field
// into the response object.
// Pass in a nil response object to skip response parsing.
// If the request fails or the server returns multiple errors, the first error
// will be returned.
func (c *Client) Run(ctx context.Context, req *Request, resp interface{}) (*http.Response, error) {
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}
	if len(req.files) > 0 && !c.useMultipartForm {
		return nil, errors.New("cannot send files with PostFields option")
	}
	if c.useMultipartForm {
		return c.runWithPostFields(ctx, req, resp)
	}
	return c.runWithJSON(ctx, req, resp)
}

func (c *Client) runWithJSON(ctx context.Context, req *Request, resp interface{}) (*http.Response, error) {
	// Build the request body
	var requestBody bytes.Buffer
	requestBodyObj := struct {
		Query     string                 `json:"query"`
		Variables map[string]interface{} `json:"variables"`
	}{
		Query:     req.query,
		Variables: req.variables,
	}
	if err := json.NewEncoder(&requestBody).Encode(requestBodyObj); err != nil {
		return nil, errors.Wrap(err, "encode body")
	}
	c.logf(">> variables: %v", req.variables)
	c.logf(">> query: %s", req.query)

	// Build the request
	gr := &graphResponse{
		Data: resp,
	}
	r, err := http.NewRequest(http.MethodPost, c.endpoint, &requestBody)
	if err != nil {
		return nil, err
	}
	r.Close = c.closeReq

	// Set the headers
	r.Header.Set("Content-Type", "application/json; charset=utf-8")
	r.Header.Set("Accept", "application/json; charset=utf-8")
	for key, values := range req.Header {
		for _, value := range values {
			r.Header.Add(key, value)
		}
	}
	c.logf(">> headers: %v", r.Header)

	// Send the request
	r = r.WithContext(ctx)
	res, err := c.httpClient.Do(r)
	if err != nil {
		return res, err
	}
	defer res.Body.Close()

	// Read the response
	var buf bytes.Buffer
	if _, err := io.Copy(&buf, res.Body); err != nil {
		return res, errors.Wrap(err, "reading body")
	}
	c.logf("<< %s", buf.String())
	if err := json.NewDecoder(&buf).Decode(&gr); err != nil {
		if res.StatusCode != http.StatusOK {
			return res, fmt.Errorf("graphql: server returned a non-200 status code: %v", res.StatusCode)
		}
		return res, errors.Wrap(err, "decoding response")
	}
	if len(gr.Errors) > 0 {
		// return first error for now
		return res, gr.Errors[0]
	}
	return res, nil
}

func (c *Client) runWithPostFields(ctx context.Context, req *Request, resp interface{}) (*http.Response, error) {
	// Build the multipart request body
	var requestBody bytes.Buffer
	writer := multipart.NewWriter(&requestBody)
	if err := writer.WriteField("query", req.query); err != nil {
		return nil, errors.Wrap(err, "write query field")
	}
	var variablesBuf bytes.Buffer
	if len(req.variables) > 0 {
		variablesField, err := writer.CreateFormField("variables")
		if err != nil {
			return nil, errors.Wrap(err, "create variables field")
		}
		if err := json.NewEncoder(io.MultiWriter(variablesField, &variablesBuf)).Encode(req.variables); err != nil {
			return nil, errors.Wrap(err, "encode variables")
		}
	}

	// Add files
	for i := range req.files {
		part, err := writer.CreateFormFile(req.files[i].Field, req.files[i].Name)
		if err != nil {
			return nil, errors.Wrap(err, "create form file")
		}
		if _, err := io.Copy(part, req.files[i].R); err != nil {
			return nil, errors.Wrap(err, "preparing file")
		}
	}
	if err := writer.Close(); err != nil {
		return nil, errors.Wrap(err, "close writer")
	}
	c.logf(">> variables: %s", variablesBuf.String())
	c.logf(">> files: %d", len(req.files))
	c.logf(">> query: %s", req.query)

	// Build the request
	gr := &graphResponse{
		Data: resp,
	}
	r, err := http.NewRequest(http.MethodPost, c.endpoint, &requestBody)
	if err != nil {
		return nil, err
	}
	r.Close = c.closeReq

	// Set the headers
	r.Header.Set("Content-Type", writer.FormDataContentType())
	r.Header.Set("Accept", "application/json; charset=utf-8")
	for key, values := range req.Header {
		for _, value := range values {
			r.Header.Add(key, value)
		}
	}
	c.logf(">> headers: %v", r.Header)

	// Send the request
	r = r.WithContext(ctx)
	res, err := c.httpClient.Do(r)
	if err != nil {
		return res, err
	}
	defer res.Body.Close()

	// Read the response
	var buf bytes.Buffer
	if _, err := io.Copy(&buf, res.Body); err != nil {
		return res, errors.Wrap(err, "reading body")
	}
	c.logf("<< %s", buf.String())
	if err := json.NewDecoder(&buf).Decode(&gr); err != nil {
		if res.StatusCode != http.StatusOK {
			return res, fmt.Errorf("graphql: server returned a non-200 status code: %v", res.StatusCode)
		}
		return res, errors.Wrap(err, "decoding response")
	}
	if len(gr.Errors) > 0 {
		// return first error for now
		return res, gr.Errors[0]
	}
	return res, nil
}

// WithHTTPClient specifies the underlying http.Client to use when
// making requests.
//
//	NewClient(endpoint, WithHTTPClient(specificHTTPClient))
func WithHTTPClient(httpclient *http.Client) ClientOption {
	return func(client *Client) {
		client.httpClient = httpclient
	}
}

// UseMultipartForm uses multipart/form-data and activates support for
// files.
func UseMultipartForm() ClientOption {
	return func(client *Client) {
		client.useMultipartForm = true
	}
}

// ImmediatelyCloseReqBody will close the req body immediately after each request body is ready
func ImmediatelyCloseReqBody() ClientOption {
	return func(client *Client) {
		client.closeReq = true
	}
}

// ClientOption are functions that are passed into NewClient to
// modify the behaviour of the Client.
type ClientOption func(*Client)

type graphErr struct {
	Message string
}

func (e graphErr) Error() string {
	return "graphql: " + e.Message
}

type graphResponse struct {
	Data   interface{}
	Errors []graphErr
}

// Request is a GraphQL request struct.
type Request struct {
	query     string
	variables map[string]interface{}
	files     []File

	// Header represent any request headers that will be set
	// when the request is made.
	Header http.Header
}

// NewRequest makes a new Request with the specified string as query.
func NewRequest(query string) *Request {
	req := &Request{
		query:  query,
		Header: make(map[string][]string),
	}
	return req
}

// WithVars adds variables for a Request.
//
//	// Add the variable `username` with value `lelebus`
//	req.WithVars(map[string]interface{}{ "username": "lelebus" })
//
//	// You would use it also like this:
//	req = NewRequest(query).WithVars(variables)
func (req *Request) WithVars(variables map[string]interface{}) *Request {
	req.variables = variables
	return req
}

// File sets a file to upload.
// Files are only supported with a Client that was created with
// the UseMultipartForm option.
// 
//	client := gqlclient.NewClient(URL, gqlclient.UseMultipartForm())
func (req *Request) File(fieldname, filename string, r io.Reader) {
	req.files = append(req.files, File{
		Field: fieldname,
		Name:  filename,
		R:     r,
	})
}

// File represents a file to upload.
type File struct {
	Field string
	Name  string
	R     io.Reader
}

// HttpClient gets the underlying http.Client.
func (c *Client) HttpClient() *http.Client {
	return c.httpClient
}

// Query gets the query string of this request.
func (req *Request) Query() string {
	return req.query
}

// Vars gets the variables for this Request.
func (req *Request) Vars() map[string]interface{} {
	return req.variables
}

// Files gets the files in this request.
func (req *Request) Files() []File {
	return req.files
}

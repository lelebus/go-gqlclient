# gqlclient [![GoDoc](https://godoc.org/github.com/machinebox/graphql?status.png)](http://godoc.org/github.com/machinebox/graphql)

Low-level GraphQL client for Go, specifically thought for testing of GraphQL services.

- Simple, familiar API
- Respects `context.Context` timeouts and cancellation
- Build and execute any kind of GraphQL request
- Use strong Go types for response data
- Use variables and upload files
- Simple error handling

## Table of contents

- [Getting started](#getting-started)
- [Usage](#usage)
- [Credits](#credits)

## Getting started

Make sure you have a working Go environment. To install gqlclient, simply run:

```
$ go get github.com/lelebus/go-gqlclient
```

## Usage

```go
// create a client (safe to share across requests)
client := gqlclient.NewClient("http://localhost:4000/graphql")

// to specify your own http.Client, use the WithHTTPClient option:
customHttpClient := &http.Client{}
customClient := gqlclient.NewClient("http://localhost:4000/graphql", gqlclient.WithHTTPClient(customHttpClient))

// make a request
req := gqlclient.NewRequest(`
    query {
        items {
            field1
            field2
            field3
        }
    }
`)

// make a request with variables
req := gqlclient.NewRequest(`
    query ($key: String!) {
        items (id:$key) {
            field1
            field2
            field3
        }
    }
`).WithVars(map[string]interface{}{
    "key": "value",
})


// run it and capture the response
var responseData map[string]interface{}
res, err := client.Run(ctx, req, &responseData)
if err != nil {
    log.Fatal(err)
}

// read the cookies in the response
cookies := res.Cookies()
```

### File support via multipart form data

By default, the package will send a JSON body. To enable the sending of files, you can opt to
use multipart form data instead using the `UseMultipartForm` option when you create your `Client`:

```
client := gqlclient.NewClient("http://localhost:4000/graphql", gqlclient.UseMultipartForm())
```

For more information, [read the godoc package documentation](http://godoc.org/github.com/machinebox/graphql)

## Credits

The idea comes from [machinebox's repository](https://github.com/machinebox/graphql). Seemed like an abandoned project, so I got the basic idea and built it out further.

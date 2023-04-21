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
client := gqlclient.NewClient("https://machinebox.io/graphql")

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
var respData map[string]interface{}
if err := client.Run(ctx, req, &respData); err != nil {
    log.Fatal(err)
}

// To specify your own http.Client, use the WithHTTPClient option:
httpclient := &http.Client{}
client := gqlclient.NewClient("https://localhost:4000/graphql", gqlclient.WithHTTPClient(httpclient))
```

### File support via multipart form data

By default, the package will send a JSON body. To enable the sending of files, you can opt to
use multipart form data instead using the `UseMultipartForm` option when you create your `Client`:

```
client := graphql.NewClient("http://localhost:4000/graphql", graphql.UseMultipartForm())
```

For more information, [read the godoc package documentation](http://godoc.org/github.com/machinebox/graphql)

## Credits

The idea comes from [machinebox's repository](https://github.com/machinebox/graphql). Seemed like an abandoned project, so I got the basic idea and built it out further.

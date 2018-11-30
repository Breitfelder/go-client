// Copyright 2018 Sourced Technologies SL
// Licensed under the Apache License, Version 2.0 (the "License"); you may not
// use this file except in compliance with the License. You may obtain a copy
// of the License at
//     http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS, WITHOUT
// WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied. See the
// License for the specific language governing permissions and limitations under
// the License.

package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"

	flags "github.com/jessevdk/go-flags"
	bblfsh "gopkg.in/bblfsh/client-go.v2"
	"gopkg.in/bblfsh/client-go.v2/tools"
	protocol1 "gopkg.in/bblfsh/sdk.v1/protocol"
	"gopkg.in/bblfsh/sdk.v1/uast"
	"gopkg.in/bblfsh/sdk.v2/uast/nodes"
)

func main() {
	var opts struct {
		Host     string `short:"a" long:"host" description:"Babelfish endpoint address" default:"localhost:9432"`
		Language string `short:"l" long:"language" description:"language to parse (default: auto)"`
		Query    string `short:"q" long:"query" description:"XPath query applied to the resulting UAST"`
		Mode     string `short:"m" long:"mode" description:"UAST transformation mode: semantic, annotated, native"`
		V2       bool   `long:"v2" description:"return UAST in v2 format"`
		Out      string `short:"o" long:"out" description:"Output format: proto, json" default:"json"`
	}
	args, err := flags.Parse(&opts)
	if err != nil {
		fatalf("couldn't parse flags: %v", err)
	}

	if len(args) == 0 {
		fatalf("missing file to parse")
	} else if len(args) > 1 {
		fatalf("couldn't parse more than a file at a time")
	}
	filename := args[0]

	client, err := bblfsh.NewClient(opts.Host)
	if err != nil {
		fatalf("couldn't create client: %v", err)
	}

	var ast interface{}
	var res *protocol1.ParseResponse
	if opts.V2 {
		req := client.NewParseRequestV2().
			Language(opts.Language).
			Filename(filename).
			ReadFile(filename)
		if opts.Mode != "" {
			m, err := bblfsh.ParseMode(opts.Mode)
			if err != nil {
				fatalf("%v", err)
			}
			req = req.Mode(m)
		}
		root, _, err := req.UAST()
		if err != nil {
			fatalf("couldn't parse %s: %v", args[0], err)
		}
		if opts.Query != "" {
			var arr nodes.Array
			it, err := tools.FilterXPath(root, opts.Query)
			if err != nil {
				fatalf("couldn't apply query %q: %v", opts.Query, err)
			}
			for it.Next() {
				arr = append(arr, it.Node().(nodes.Node))
			}
			root = arr
		}
		ast = root
	} else {
		req := client.NewParseRequest().
			Language(opts.Language).
			Filename(filename).
			ReadFile(filename)
		if opts.Mode != "" {
			m, err := bblfsh.ParseMode(opts.Mode)
			if err != nil {
				fatalf("%v", err)
			}
			req = req.Mode(m)
		}
		res, err = req.Do()
		if err != nil {
			fatalf("couldn't parse %s: %v", args[0], err)
		}

		nodes := []*uast.Node{res.UAST}
		if opts.Query != "" {
			nodes, err = tools.Filter(res.UAST, opts.Query)
			if err != nil {
				fatalf("couldn't apply query %q: %v", opts.Query, err)
			}
		}
		ast = nodes
	}

	var b []byte
	switch opts.Out {
	case "", "json":
		b, err = json.MarshalIndent(ast, "", "  ")
		if err != nil {
			fatalf("couldn't encode UAST: %v", err)
		}
		fmt.Printf("%s\n", b)
	case "proto":
		if opts.V2 || opts.Query != "" {
			fatalf(".pb output format is only supported for V1 requests without any queries")
		}

		outFileName := fmt.Sprintf("./%s.pb", filepath.Base(filename))
		fmt.Fprintf(os.Stderr, "Saving result to %s\n", outFileName)

		protoUast, err := res.UAST.Marshal()
		if err != nil {
			fatalf("failed to encode UAST to Protobuf: %v", err)
		}

		ioutil.WriteFile(outFileName, []byte(protoUast), 0644)
		if err != nil {
			fatalf("failed to write Protobuf to %s, %v", outFileName, err)
		}
	default:
		fatalf("unsupported output format: %q", opts.Out)
	}
}

func fatalf(msg string, args ...interface{}) {
	fmt.Fprintf(os.Stderr, msg+"\n", args...)
	os.Exit(1)
}

package http

import (
	"bytes"
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"unsafe"

	"github.com/gorilla/mux"
	"github.com/jbussdieker/golibxml"
	"github.com/krolaw/xsd"
	"github.com/xeipuuv/gojsonschema"
)

// MatcherBySchema check if the request matching with the schema file
func MatcherBySchema(imposter Imposter) mux.MatcherFunc {
	return func(req *http.Request, rm *mux.RouteMatch) bool {
		if imposter.Request.SchemaFile == nil {
			return true
		}

		var err error
		switch filepath.Ext(*imposter.Request.SchemaFile) {
		case ".json":
			err = validateJSONSchema(imposter, req)
		case ".xml", ".xsd":
			err = validateXMLSchema(imposter, req)
		default:
			err = errors.New("unknown schema file extension")
		}

		// TODO: inject the logger
		if err != nil {
			log.Println(err)
			return false
		}
		return true
	}
}

func validateJSONSchema(imposter Imposter, req *http.Request) error {
	var requestBodyBytes []byte

	defer func() {
		req.Body.Close()
		req.Body = ioutil.NopCloser(bytes.NewBuffer(requestBodyBytes))
	}()

	schemaFile := imposter.CalculateFilePath(*imposter.Request.SchemaFile)
	if _, err := os.Stat(schemaFile); os.IsNotExist(err) {
		return fmt.Errorf("%w: the schema file %s not found", err, schemaFile)
	}

	requestBodyBytes, err := ioutil.ReadAll(req.Body)
	if err != nil {
		return fmt.Errorf("%w: error reading the request body", err)
	}

	contentBody := string(requestBodyBytes)
	if contentBody == "" {
		return fmt.Errorf("unexpected empty body request")
	}

	schemaFilePath, err := filepath.Abs(schemaFile)
	if err != nil {
		return fmt.Errorf("%w: error finding the schema file", err)
	}

	schemaBytes, err := ioutil.ReadFile(schemaFilePath)
	if err != nil {
		return fmt.Errorf("%w: error reading the schema file", err)
	}

	schema := gojsonschema.NewStringLoader(string(schemaBytes))
	document := gojsonschema.NewStringLoader(string(requestBodyBytes))

	res, err := gojsonschema.Validate(schema, document)
	if err != nil {
		return fmt.Errorf("%w: error validating the json schema", err)
	}

	if !res.Valid() {
		for _, desc := range res.Errors() {
			return errors.New(desc.String())
		}
	}

	return nil
}

func validateXMLSchema(imposter Imposter, req *http.Request) error {
	var requestBodyBytes []byte

	defer func() {
		req.Body.Close()
		req.Body = ioutil.NopCloser(bytes.NewBuffer(requestBodyBytes))
	}()

	schemaFile := imposter.CalculateFilePath(*imposter.Request.SchemaFile)
	if _, err := os.Stat(schemaFile); os.IsNotExist(err) {
		return fmt.Errorf("%w: xsd file %s not found", err, schemaFile)
	}

	requestBodyBytes, err := ioutil.ReadAll(req.Body)
	if err != nil {
		return fmt.Errorf("%w: error reading the request body", err)
	}

	contentBody := string(requestBodyBytes)
	if contentBody == "" {
		return fmt.Errorf("unexpected empty body request")
	}

	// Following the example from the official documentation:
	// https://godoc.org/github.com/krolaw/xsd
	schemaFilePath, err := filepath.Abs(schemaFile)
	if err != nil {
		return fmt.Errorf("%w: error finding the schema file", err)
	}

	schemaBytes, err := ioutil.ReadFile(schemaFilePath)
	if err != nil {
		return fmt.Errorf("%w: error reading the schema file", err)
	}

	schema, err := xsd.ParseSchema(schemaBytes)
	if err != nil {
		return fmt.Errorf("%w: error parsing xsd schema", err)
	}

	document := golibxml.ParseDoc(string(requestBodyBytes))
	defer document.Free()

	if err := schema.Validate(xsd.DocPtr(unsafe.Pointer(document.Ptr))); err != nil {
		return fmt.Errorf("%w: error validating the xml request", err)
	}

	return nil
}

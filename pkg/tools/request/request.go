package request

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"time"

	"github.com/go-errors/errors"
	"go.uber.org/zap"

	"github.com/cisco-eti/ioc-cfn-svc/pkg/tools/logger"
)

var l *zap.SugaredLogger

func getLogger() *zap.SugaredLogger {
	if l == nil {
		l = logger.SubPkg("app")
	}
	return l
}

func DoJSONRequest(method, url string, optHeaders http.Header,
	payload io.Reader, respPayload interface{}, skipStatusCheck bool) (int,
	error) {

	log := getLogger()

	headers := http.Header(make(map[string][]string))
	headers.Add("Content-Type", "application/json")

	for k, vals := range optHeaders {
		for _, v := range vals {
			headers.Add(k, v)
		}
	}

	res, err := DoRequest(method, url, headers, payload)
	if err != nil {
		return 500, err
	}
	defer res.Body.Close()

	// expects 200s. not really 300s, but shouldn't error
	if !skipStatusCheck && res.StatusCode >= 400 {
		reqBody, resBody := bestEffortReadBodies(payload, res)
		log.Warnf("[%s %s] [%v] failed with status [%s]. sent [%s]. received [%s].",
			method, url, headers, res.Status, reqBody, resBody)
		return res.StatusCode,
			errors.Errorf("unexpected status received: %s", res.Status)
	}

	if respPayload != nil {
		if res.Body == nil {
			return res.StatusCode,
				errors.Errorf("expected response body, but got empty body")
		}

		err = json.NewDecoder(res.Body).Decode(respPayload)
		if err != nil {
			return res.StatusCode, errors.New(err)
		}
	}

	return res.StatusCode, nil
}

func bestEffortReadBodies(payload io.Reader, res *http.Response) (string,
	string) {

	reqBody := ""
	if payload != nil {
		reqBodyBin, readErr := io.ReadAll(payload)
		if readErr == nil { // reverse logic
			reqBody = logger.Trunc(string(reqBodyBin), 200)
		}
	}

	resBody := ""
	resBodyBin, readErr := io.ReadAll(res.Body)
	if readErr == nil { // reverse logic
		resBody = logger.Trunc(string(resBodyBin), 200)
	}

	return reqBody, resBody
}

const (
	expRetryCount         = 7 // exponential retry
	expRetryBackoffFactor = time.Second
)

func DoRequest(method, url string, headers http.Header, payload io.Reader) (
	*http.Response, error) {
	log := getLogger()

	// create a copy of the io reader so that it can be reused on retries
	var payloadCopyBuf bytes.Buffer
	if payload != nil {
		_, err := io.Copy(&payloadCopyBuf, payload)
		if err != nil {
			return nil, errors.New(err)
		}
	}

	var res *http.Response
	for i := 0; i < expRetryCount; i++ {
		var payloadCopy io.Reader
		if payload != nil {
			payloadCopy = bytes.NewReader(payloadCopyBuf.Bytes())
		}

		req, err := http.NewRequest(method, url, payloadCopy)
		if err != nil {
			return nil, errors.New(err)
		}

		log.Infof("making [%s] request to [%s]", method, url)

		req.Header = headers
		res, err = http.DefaultClient.Do(req)
		if err != nil {
			return nil, errors.New(err)
		}
		log.Infof("--> [%s]", res.Status)

		if res.StatusCode != http.StatusBadGateway &&
			res.StatusCode != http.StatusServiceUnavailable &&
			res.StatusCode != http.StatusGatewayTimeout {
			// break out of this loop for anything other than 502, 503, 504
			break
		}

		if i+1 < expRetryCount {
			backoff := time.Duration(i*i) * expRetryBackoffFactor
			log.Infof("retying request [%s] [%s] after backoff [%d] [%s]", method,
				url, i+1, backoff)
			time.Sleep(backoff)
		}
	}

	return res, nil
}

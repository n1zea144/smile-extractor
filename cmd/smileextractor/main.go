package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/mskcc/smile-extractor/internal/csv"
	"github.com/mskcc/smile-extractor/internal/smile"
	"github.com/spf13/pflag"
	"github.com/spf13/viper"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"sync"
	"time"
)

const (
	dForm         = "2006-01-02"
	rByImportDate = "http://smile.mskcc.org:3000/requestsByImportDate?returnType=REQUEST_ID_LIST"
	rById         = "http://smile.mskcc.org:3000/request/"
	timeout       = 30 * time.Minute
	requestFile   = ""
	sampleFile    = ""
)

func setupOptions() {
	pflag.StringP("r_date", "d", "", "Request import date")
	pflag.Parse()
	viper.BindPFlags(pflag.CommandLine)
}

func parseArgs() (time.Time, error) {
	if viper.GetBool("help") {
		pflag.PrintDefaults()
		os.Exit(0)
	}
	t := time.Now()
	rd := viper.GetString("r_date")
	if rd == "" {
		return t, errors.New("Missing r_date argument")
	}
	t, err := time.Parse(dForm, rd)
	if err != nil {
		return t, err
	}
	return t, nil
}

func getRequestIds(rDate time.Time) ([]string, error) {
	body := fmt.Sprintf("{\"startDate\":\"%s\"}", rDate.Format(dForm))
	bodyReader := bytes.NewReader([]byte(body))

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, rByImportDate, bodyReader)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	httpClient := &http.Client{}
	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, err
	}

	defer resp.Body.Close()
	rBody, _ := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP return code != 200: %d\n", resp.StatusCode)
	}

	var rIds []string
	if err := json.Unmarshal(rBody, &rIds); err != nil {
		return nil, err
	}

	return rIds, nil
}

func removeDuplicateValues(slice []string) []string {
	keys := make(map[string]bool)
	list := []string{}

	for _, entry := range slice {
		if _, value := keys[entry]; !value {
			keys[entry] = true
			list = append(list, entry)
		}
	}
	return list
}

func getRequest(rId string, requestCh chan smile.Request, tokens chan struct{}, wg *sync.WaitGroup) {

	defer wg.Done()
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	url := fmt.Sprintf("%s%s", rById, rId)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return
	}
	req.Header.Set("Accept", "application/json")
	httpClient := &http.Client{}
	resp, err := httpClient.Do(req)
	if err != nil {
		return
	}

	defer resp.Body.Close()
	rBody, _ := ioutil.ReadAll(resp.Body)
	if err != nil {
		return
	}
	if resp.StatusCode != http.StatusOK {
		return
	}

	var r smile.Request
	if err := json.Unmarshal(rBody, &r); err != nil {
		return
	}
	requestCh <- r
	<-tokens
}

func main() {
	setupOptions()
	rDate, err := parseArgs()
	if err != nil {
		log.Fatal("failed to parse arguments: ", err)
	}

	rIds, err := getRequestIds(rDate)
	if err != nil {
		log.Fatal("Error getting request ids: ", err)
	}
	rIds = removeDuplicateValues(rIds)

	requestCh := make(chan smile.Request, 10)
	tokensCh := make(chan struct{}, 10)
	var wgCSV sync.WaitGroup
	wgCSV.Add(1)
	go csv.AddRequest(requestFile, sampleFile, requestCh, &wgCSV)

	var wgGet sync.WaitGroup

	for _, rId := range rIds {
		wgGet.Add(1)
		tokensCh <- struct{}{}
		go getRequest(rId, requestCh, tokensCh, &wgGet)
	}
	wgGet.Wait()
	close(requestCh)
	wgCSV.Wait()
	log.Println("Exiting...")
}

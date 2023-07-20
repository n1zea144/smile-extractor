package csv

import (
	"encoding/json"
	"fmt"
	"github.com/mskcc/smile-extractor/internal/smile"
	"os"
	"sync"
)

func AddRequest(rf string, sf string, requestCh chan smile.Request, wg *sync.WaitGroup) {
	defer wg.Done()
	for sr := range requestCh {
		// lets save samples first, because we want to remove them from request before saving request
		// its also more likely that we will encounter an error here than when saving a request because
		// 1 request -> 1 or more samples
		err := insertSamples(sf, sr)
		if err != nil {
			return
		}

		err = insertRequest(rf, sr)
		if err != nil {
			return
		}
	}
}

func insertSamples(sf string, sr smile.Request) error {
	f, err := os.OpenFile(sf, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	for _, s := range sr.Samples {
		sJson, err := json.Marshal(s)
		if err != nil {
			return err
		}
		record := fmt.Sprintf("%s\t%s\t%s\t%s\t%s\n", sr.IgoRequestID, s.SampleName, s.CmoSampleName, s.CFDNA2DBarcode, string(sJson))
		_, err = f.WriteString(record)
		if err != nil {
			return err
		}
	}
	f.Close()
	return nil
}

func insertRequest(rf string, sr smile.Request) error {
	f, err := os.OpenFile(rf, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}

	// clobber samples in sr,Samples[] before saving because they just got stored in the samples table
	sr.Samples = sr.Samples[:0]
	rJson, err := json.Marshal(sr)
	if err != nil {
		return err
	}
	record := fmt.Sprintf("%s\t%s\n", sr.IgoRequestID, string(rJson))
	_, err = f.WriteString(record)
	if err != nil {
		return err
	}
	f.Close()
	return nil
}

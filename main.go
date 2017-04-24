package main

import (
	"fmt"
	"github.com/apex/log"
	"github.com/apex/log/handlers/cli"
	"gopkg.in/alecthomas/kingpin.v2"
	"net/http"
	"os"
	"sync"
	"time"
)

const GmailAPIBaseURL string = "https://mail.google.com/mail/gxlu?email="

var (
	app = kingpin.New("gmail_address_verifier", "Query Gmail to see if an email address is valid.")

	noOfWorkers = app.Flag("workers", "number of workers to run").Short('w').Default("2").Int()
	debug       = app.Flag("debug", "print debug info").Short('d').Bool()
	emails      = app.Arg("email address", "the email address(es) to lookup.").Required().Strings()
)

var wg sync.WaitGroup

func main() {

	app.Version("0.1").VersionFlag.Short('V')
	app.HelpFlag.Short('h')
	app.UsageTemplate(kingpin.SeparateOptionalFlagsUsageTemplate)
	kingpin.MustParse(app.Parse(os.Args[1:]))

	log.SetHandler(cli.New(os.Stdout))
	log.SetLevel(log.InfoLevel)

	if *debug {
		log.SetLevel(log.DebugLevel)
	}

	jobQ := make(chan string, 10)

	// start concurrently processing lines from the jobQ
	for i := 0; i < *noOfWorkers; i++ {
		wg.Add(1)
		go verifyEmail(jobQ, &wg)
	}

	// load up the job queue with the provided emails
	for _, email := range *emails {
		jobQ <- email
		log.Debugf("added %s to job queue", email)
	}

	close(jobQ)

	wg.Wait()

}

func verifyEmail(jobQueue <-chan string, wg *sync.WaitGroup) {
	defer wg.Done()

	for email := range jobQueue {

		client := &http.Client{
			Timeout: 15 * time.Second,
		}

		url := GmailAPIBaseURL + email

		req, err := http.NewRequest("GET", url, nil)
		if err != nil {
			log.WithError(err).Errorf("Error creating verification request for %s", email)
		}

		//add some browser headers for stealth
		for k, v := range commonHeaders() {
			req.Header.Add(k, v)
		}

		log.Debugf("issuing query for %s", email)

		resp, err := client.Do(req)
		if err != nil {
			log.WithError(err).Errorf("Error verifiying email address %s", email)
		}

		headers := resp.Header

		if len(headers.Get("Set-Cookie")) > 0 {
			//Gmail has disclosed the email address exits
			printValid("%s is %s", email, "valid")
		} else {
			//email address does not exist
			printInvalid("%s is %s", email, "invalid")
		}
	}

}

func printValid(format string, values ...interface{}) {

	message := fmt.Sprintf(format, values...)

	fmt.Fprintf(os.Stdout, "\033[32m%*s\033[0m %-25s\n", 4, "✓", message)
}

func printInvalid(format string, values ...interface{}) {

	message := fmt.Sprintf(format, values...)

	fmt.Fprintf(os.Stdout, "\033[31m%*s\033[0m %-25s\n", 4, "⨯", message)
}

func commonHeaders() map[string]string {
	return map[string]string{
		"Accept":          "text/html,application/xhtml+xml,application/xml;q=0.9,image/webp,*/*;q=0.8",
		"User-Agent":      "Mozilla/5.0 (Windows NT 6.1; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/56.0.2924.87 Safari/537.36",
		"Accept-Encoding": "gzip, deflate, sdch, br",
		"Accept-Language": "en-US,en;q=0.8",
	}
}

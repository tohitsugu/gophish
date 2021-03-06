package worker

import (
	"bytes"
	"log"
	"net"
	"net/smtp"
	"os"
	"strings"
	"text/template"
	"time"

	"github.com/jordan-wright/email"
	"github.com/jordan-wright/gophish/models"
)

// Logger is the logger for the worker
var Logger = log.New(os.Stdout, " ", log.Ldate|log.Ltime|log.Lshortfile)

// Worker is the background worker that handles watching for new campaigns and sending emails appropriately.
type Worker struct {
	Queue chan *models.Campaign
}

// New creates a new worker object to handle the creation of campaigns
func New() *Worker {
	return &Worker{
		Queue: make(chan *models.Campaign),
	}
}

// Start launches the worker to monitor the database for any jobs.
// If a job is found, it launches the job
func (w *Worker) Start() {
	Logger.Println("Background Worker Started Successfully - Waiting for Campaigns")
	for {
		processCampaign(<-w.Queue)
	}
}

func processCampaign(c *models.Campaign) {
	Logger.Printf("Worker received: %s", c.Name)
	err := c.UpdateStatus(models.CAMPAIGN_IN_PROGRESS)
	if err != nil {
		Logger.Println(err)
	}
	e := email.Email{
		Subject: c.Template.Subject,
		From:    c.SMTP.FromAddress,
	}
	var auth smtp.Auth
	if c.SMTP.Username != "" && c.SMTP.Password != "" {
		auth = smtp.PlainAuth("", c.SMTP.Username, c.SMTP.Password, strings.Split(c.SMTP.Host, ":")[0])
	}
	ips, err := net.InterfaceAddrs()
	if err != nil {
		Logger.Println(err)
	}
	for _, i := range ips {
		Logger.Println(i.String())
	}
	for _, t := range c.Results {
		td := struct {
			models.Result
			URL     string
			Tracker string
		}{
			t,
			"http://" + ips[0].String() + "?rid=" + t.RId,
			"http://" + ips[0].String() + "/track?rid=" + t.RId,
		}
		// Parse the templates
		var subjBuff bytes.Buffer
		var htmlBuff bytes.Buffer
		var textBuff bytes.Buffer
		tmpl, err := template.New("html_template").Parse(c.Template.HTML)
		if err != nil {
			Logger.Println(err)
		}
		err = tmpl.Execute(&htmlBuff, td)
		if err != nil {
			Logger.Println(err)
		}
		e.HTML = htmlBuff.Bytes()
		tmpl, err = template.New("text_template").Parse(c.Template.Text)
		if err != nil {
			Logger.Println(err)
		}
		err = tmpl.Execute(&textBuff, td)
		if err != nil {
			Logger.Println(err)
		}
		e.Text = textBuff.Bytes()
		tmpl, err = template.New("text_template").Parse(c.Template.Subject)
		if err != nil {
			Logger.Println(err)
		}
		err = tmpl.Execute(&subjBuff, td)
		if err != nil {
			Logger.Println(err)
		}
		e.Subject = string(subjBuff.Bytes())
		Logger.Println("Creating email using template")
		e.To = []string{t.Email}
		Logger.Printf("Sending Email to %s\n", t.Email)
		err = e.Send(c.SMTP.Host, auth)
		if err != nil {
			Logger.Println(err)
			err = t.UpdateStatus("Error")
			if err != nil {
				Logger.Println(err)
			}
		} else {
			err = t.UpdateStatus(models.EVENT_SENT)
			if err != nil {
				Logger.Println(err)
			}
		}
		time.Sleep(1)
	}
}

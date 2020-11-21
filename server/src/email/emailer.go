package email

import (
    "fmt"
    "local/hansa/log"
    "local/hansa/simple"
    "github.com/aws/aws-sdk-go/aws"
    "github.com/aws/aws-sdk-go/service/ses"
    "github.com/aws/aws-sdk-go/aws/awserr"
)

type Emailer struct {
    config simple.Config
    svc *ses.SES
    sendChan chan Email 
}

type Email struct {
    Address string
    Subject string
    Text string
    Html string
}

func NewEmailer(config simple.Config) *Emailer {
    return &Emailer{
        config: config,
        svc: ses.New(config.Session),
        sendChan: make(chan Email, 10),
    }
}

func (e *Emailer) Run(initDone chan struct{}) {
    e.debugf("Emailer Running")
    initDone <-struct{}{}
    for email := range e.sendChan {
        e.send(email)
    }
    e.debugf("Emailer Done")
}

func (e *Emailer) Done() {
    close(e.sendChan)
}

func (e *Emailer) Send(addr string, subject string, text string, html string) {
    e.sendChan <-Email{
        Address: addr,
        Subject: subject,
        Text: text,
        Html: html,
    }
}

func (e *Emailer) send(email Email) {
    sender := e.config.EmailSender
    e.debugf("Sending Email From : %s To: %s, subject: %s",
        sender, email.Address, email.Subject)

    if e.config.Name != "prod" {
        e.infof("Rewriting addr to corykendall@gmail.com as we are beta.")
        email.Text = fmt.Sprintf("EmailPreCorykenRedirect: %s\n%s", email.Address, email.Text)
        email.Html = fmt.Sprintf("EmailPreCorykenRedirect: %s\n%s", email.Address, email.Html)
        email.Address = "corykendall@gmail.com"
    }

    input := &ses.SendEmailInput{
        Destination: &ses.Destination{
            CcAddresses: []*string{},
            ToAddresses: []*string{aws.String(email.Address)},
        },
        Message: &ses.Message{
            Body: &ses.Body{
                Html: &ses.Content{
                    Charset: aws.String("UTF-8"),
                    Data:    aws.String(email.Html),
                },
                Text: &ses.Content{
                    Charset: aws.String("UTF-8"),
                    Data:    aws.String(email.Text),
                },
            },
            Subject: &ses.Content{
                Charset: aws.String("UTF-8"),
                Data:    aws.String(email.Subject),
            },
        },
        Source: aws.String(sender),
    }

    // Attempt to send the email.
    result, err := e.svc.SendEmail(input)

    // Display error messages if they occur.
    if err != nil {
        if aerr, ok := err.(awserr.Error); ok {
            switch aerr.Code() {
            case ses.ErrCodeMessageRejected:
                e.errorf(ses.ErrCodeMessageRejected, aerr.Error())
            case ses.ErrCodeMailFromDomainNotVerifiedException:
                e.errorf(ses.ErrCodeMailFromDomainNotVerifiedException, aerr.Error())
            case ses.ErrCodeConfigurationSetDoesNotExistException:
                e.errorf(ses.ErrCodeConfigurationSetDoesNotExistException, aerr.Error())
            default:
                e.errorf(aerr.Error())
            }
        } else {
            // Print the error, cast err to awserr.Error to get the Code and
            // Message from an error.
            e.errorf(err.Error())
        }
        return
    }

    e.debugf(fmt.Sprintf("Email Sent to address: %s, result: %s", email.Address, result))
}

func (e *Emailer) debugf(msg string, fargs ...interface{}) {
    log.Debug(msg, fargs...)
}

func (e *Emailer) infof(msg string, fargs ...interface{}) {
    log.Info(msg, fargs...)
}

func (e *Emailer) errorf(msg string, fargs ...interface{}) {
    log.Error(msg, fargs...)
}

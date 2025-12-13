package service

import (
    "context"
    "crypto/tls"
    "fmt"
    "net/smtp"
    "strings"
    "time"

    "coin-alert/internal/domain"
    "coin-alert/internal/repository"
)

type EmailAlertService struct {
    EmailAlertRepository repository.EmailAlertRepository
    SenderAddress        string
    SenderPassword       string
    SMTPHost             string
    SMTPPort             int
}

func NewEmailAlertService(emailAlertRepository repository.EmailAlertRepository, senderAddress string, senderPassword string, smtpHost string, smtpPort int) *EmailAlertService {
    return &EmailAlertService{
        EmailAlertRepository: emailAlertRepository,
        SenderAddress:        senderAddress,
        SenderPassword:       senderPassword,
        SMTPHost:             smtpHost,
        SMTPPort:             smtpPort,
    }
}

func (service *EmailAlertService) SendAndLogAlert(contextWithTimeout context.Context, alert domain.EmailAlert) (int64, error) {
    if validationError := service.validateAlert(alert); validationError != nil {
        return 0, validationError
    }

    sendError := service.dispatchEmail(alert)
    if sendError != nil {
        return 0, sendError
    }

    alert.Identifier = 0
    alert.CreatedAt = time.Now()
    return service.EmailAlertRepository.LogEmailAlert(contextWithTimeout, alert)
}

func (service *EmailAlertService) validateAlert(alert domain.EmailAlert) error {
    if alert.RecipientAddress == "" {
        return fmt.Errorf("recipient address must be provided")
    }
    if alert.Subject == "" {
        return fmt.Errorf("subject must be provided")
    }
    if alert.MessageBody == "" {
        return fmt.Errorf("message body must be provided")
    }
    return nil
}

func (service *EmailAlertService) dispatchEmail(alert domain.EmailAlert) error {
    if service.SenderAddress == "" || service.SenderPassword == "" || service.SMTPHost == "" || service.SMTPPort == 0 {
        return fmt.Errorf("email credentials are not configured")
    }

    smtpServerAddress := fmt.Sprintf("%s:%d", service.SMTPHost, service.SMTPPort)
    authentication := smtp.PlainAuth("", service.SenderAddress, service.SenderPassword, service.SMTPHost)

    messageHeaders := []string{
        fmt.Sprintf("From: %s", service.SenderAddress),
        fmt.Sprintf("To: %s", alert.RecipientAddress),
        fmt.Sprintf("Subject: %s", alert.Subject),
        "MIME-Version: 1.0",
        "Content-Type: text/plain; charset=\"utf-8\"",
        "",
    }
    messageBody := strings.Join(messageHeaders, "\r\n") + alert.MessageBody

    tlsConfiguration := &tls.Config{ServerName: service.SMTPHost}
    connection, connectionError := tls.Dial("tcp", smtpServerAddress, tlsConfiguration)
    if connectionError != nil {
        return connectionError
    }
    defer connection.Close()

    smtpClient, smtpError := smtp.NewClient(connection, service.SMTPHost)
    if smtpError != nil {
        return smtpError
    }
    defer smtpClient.Close()

    if authenticationError := smtpClient.Auth(authentication); authenticationError != nil {
        return authenticationError
    }

    if senderError := smtpClient.Mail(service.SenderAddress); senderError != nil {
        return senderError
    }

    if recipientError := smtpClient.Rcpt(alert.RecipientAddress); recipientError != nil {
        return recipientError
    }

    dataWriter, dataError := smtpClient.Data()
    if dataError != nil {
        return dataError
    }

    _, writeError := dataWriter.Write([]byte(messageBody))
    if writeError != nil {
        return writeError
    }

    closeError := dataWriter.Close()
    if closeError != nil {
        return closeError
    }

    return smtpClient.Quit()
}

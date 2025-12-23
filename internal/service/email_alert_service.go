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

func (service *EmailAlertService) CreateAlertDefinition(contextWithTimeout context.Context, alert domain.EmailAlert) (int64, error) {
	if validationError := service.validateAlertDefinition(alert); validationError != nil {
		return 0, validationError
	}

	alert.Identifier = 0
	alert.CreatedAt = time.Now()
	alert.IsActive = true
	return service.EmailAlertRepository.CreateAlertDefinition(contextWithTimeout, alert)
}

func (service *EmailAlertService) TriggerAlert(contextWithTimeout context.Context, alert domain.EmailAlert, currentPrice float64, triggerBoundary string) error {
	if validationError := service.validateAlertDefinition(alert); validationError != nil {
		return validationError
	}
	if currentPrice <= 0 {
		return fmt.Errorf("current price must be greater than zero")
	}

	generatedSubject, generatedMessage := service.populateGeneratedContent(alert, currentPrice, triggerBoundary)
	sendError := service.dispatchEmail(alert, generatedSubject, generatedMessage)
	if sendError != nil {
		return sendError
	}

	return service.EmailAlertRepository.MarkAlertTriggered(contextWithTimeout, alert.Identifier)
}

func (service *EmailAlertService) validateAlertDefinition(alert domain.EmailAlert) error {
	if alert.RecipientAddress == "" {
		return fmt.Errorf("recipient address must be provided")
	}
	if alert.TradingPairOrCurrency == "" {
		return fmt.Errorf("trading pair or currency must be provided")
	}
	if alert.MinimumThreshold <= 0 || alert.MaximumThreshold <= 0 {
		return fmt.Errorf("minimum and maximum thresholds must be greater than zero")
	}
	if alert.MinimumThreshold >= alert.MaximumThreshold {
		return fmt.Errorf("minimum threshold must be lower than maximum threshold")
	}
	return nil
}

func (service *EmailAlertService) populateGeneratedContent(alert domain.EmailAlert, currentPrice float64, triggerBoundary string) (string, string) {
	generatedSubject := fmt.Sprintf("Price alert for %s", alert.TradingPairOrCurrency)
	generatedBody := fmt.Sprintf(
		"The price for %s has reached your %s threshold.\n\nCurrent price: %.6f\nMinimum threshold: %.6f\nMaximum threshold: %.6f\n\nThis alert is now marked as triggered.",
		alert.TradingPairOrCurrency,
		triggerBoundary,
		currentPrice,
		alert.MinimumThreshold,
		alert.MaximumThreshold,
	)
	return generatedSubject, generatedBody
}

func (service *EmailAlertService) dispatchEmail(alert domain.EmailAlert, generatedSubject string, generatedBody string) error {
        if service.SenderAddress == "" || service.SenderPassword == "" || service.SMTPHost == "" || service.SMTPPort == 0 {
                return fmt.Errorf("email credentials are not configured")
        }

	smtpServerAddress := fmt.Sprintf("%s:%d", service.SMTPHost, service.SMTPPort)
	authentication := smtp.PlainAuth("", service.SenderAddress, service.SenderPassword, service.SMTPHost)

        messageHeaders := []string{
                fmt.Sprintf("From: %s", service.SenderAddress),
                fmt.Sprintf("To: %s", alert.RecipientAddress),
                fmt.Sprintf("Subject: %s", generatedSubject),
                "MIME-Version: 1.0",
                "Content-Type: text/plain; charset=\"utf-8\"",
                "",
        }
        messageBody := strings.Join(messageHeaders, "\r\n") + generatedBody

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

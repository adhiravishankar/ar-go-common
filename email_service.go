package common

import (
	"context"
	"fmt"
	"html/template"
	"log"
	"os"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/ses"
	"github.com/aws/aws-sdk-go-v2/service/ses/types"
)

var sesClient *ses.Client

// InitializeSES initializes the SES client
func InitializeSES() error {
	cfg, err := config.LoadDefaultConfig(context.TODO())
	if err != nil {
		return fmt.Errorf("failed to load AWS config: %w", err)
	}

	sesClient = ses.NewFromConfig(cfg)
	return nil
}

// EmailTemplate represents an email template
type EmailTemplate struct {
	Subject string
	Body    string
}

// GetVerificationEmailTemplate returns the email verification template
func GetVerificationEmailTemplate(name, templateName, verificationToken string) EmailTemplate {
	subject := "Verify Your Email - Flight History App"

	// Get the base URL from environment variable
	baseURL := os.Getenv("FRONTEND_URL")
	if baseURL == "" {
		baseURL = "http://localhost:5174" // Default for development
	}

	verificationLink := fmt.Sprintf("%s/verify-email?token=%s", baseURL, verificationToken)

	body, err := template.ParseFiles(templateName)
	if err != nil {
		log.Printf("Failed to parse verification email template: %v", err)
		return EmailTemplate{}
	}

	var bodyString strings.Builder
	err = body.Execute(&bodyString, map[string]string{
		"Name":              name,
		"VerificationToken": verificationToken,
		"VerificationLink":  verificationLink,
	})

	if err != nil {
		log.Printf("Failed to execute verification email template: %v", err)
		return EmailTemplate{}
	}

	return EmailTemplate{
		Subject: subject,
		Body:    bodyString.String(),
	}
}

// SendVerificationEmail sends an email verification email using SES
func SendVerificationEmail(toEmail, name, templateName, verificationToken string) error {
	if sesClient == nil {
		return fmt.Errorf("SES client not initialized")
	}

	// Get the base URL from environment variable
	baseURL := os.Getenv("FRONTEND_URL")
	if baseURL == "" {
		baseURL = "http://localhost:5174" // Default for development
	}

	template := GetVerificationEmailTemplate(name, templateName, verificationToken)

	// Get the sender email from environment variable
	fromEmail := os.Getenv("SES_FROM_EMAIL")
	if fromEmail == "" {
		return fmt.Errorf("SES_FROM_EMAIL environment variable not set")
	}

	input := &ses.SendEmailInput{
		Destination: &types.Destination{
			ToAddresses: []string{toEmail},
		},
		Message: &types.Message{
			Subject: &types.Content{
				Data:    aws.String(template.Subject),
				Charset: aws.String("UTF-8"),
			},
			Body: &types.Body{
				Html: &types.Content{
					Data:    aws.String(template.Body),
					Charset: aws.String("UTF-8"),
				},
			},
		},
		Source: aws.String(fromEmail),
	}

	_, err := sesClient.SendEmail(context.TODO(), input)
	if err != nil {
		log.Printf("Failed to send verification email to %s: %v", toEmail, err)
		return fmt.Errorf("failed to send verification email: %w", err)
	}

	log.Printf("Verification email sent successfully to %s", toEmail)
	return nil
}

// SendWelcomeEmail sends a welcome email after successful verification
func SendWelcomeEmail(toEmail, name string) error {
	if sesClient == nil {
		return fmt.Errorf("SES client not initialized")
	}

	fromEmail := os.Getenv("SES_FROM_EMAIL")
	if fromEmail == "" {
		return fmt.Errorf("SES_FROM_EMAIL environment variable not set")
	}

	subject := "Welcome to Flight History App!"
	bodyTemplate, err := template.ParseFiles("templates/verify.html")
	if err != nil {
		log.Printf("Failed to parse welcome email template: %v", err)
		return fmt.Errorf("failed to parse welcome email template: %w", err)
	}

	var bodyString strings.Builder
	err = bodyTemplate.Execute(&bodyString, map[string]string{
		"Name":             name,
		"VerificationLink": "", // No verification link needed for welcome email
	})
	if err != nil {
		log.Printf("Failed to execute welcome email template: %v", err)
		return fmt.Errorf("failed to execute welcome email template: %w", err)
	}

	input := &ses.SendEmailInput{
		Destination: &types.Destination{
			ToAddresses: []string{toEmail},
		},
		Message: &types.Message{
			Subject: &types.Content{
				Data:    aws.String(subject),
				Charset: aws.String("UTF-8"),
			},
			Body: &types.Body{
				Html: &types.Content{
					Data:    aws.String(bodyString.String()),
					Charset: aws.String("UTF-8"),
				},
			},
		},
		Source: aws.String(fromEmail),
	}

	_, err = sesClient.SendEmail(context.TODO(), input)
	if err != nil {
		log.Printf("Failed to send welcome email to %s: %v", toEmail, err)
		return fmt.Errorf("failed to send welcome email: %w", err)
	}

	log.Printf("Welcome email sent successfully to %s", toEmail)
	return nil
}

// SendPasswordResetEmail sends a password reset email using SES
func SendPasswordResetEmail(toEmail, name, resetToken string) error {
	if sesClient == nil {
		return fmt.Errorf("SES client not initialized")
	}

	// Get the base URL from environment variable
	baseURL := os.Getenv("FRONTEND_URL")
	if baseURL == "" {
		baseURL = "http://localhost:5174" // Default for development
	}

	resetLink := fmt.Sprintf("%s/reset-password?token=%s", baseURL, resetToken)

	// Get the sender email from environment variable
	fromEmail := os.Getenv("SES_FROM_EMAIL")
	if fromEmail == "" {
		return fmt.Errorf("SES_FROM_EMAIL environment variable not set")
	}

	subject := "Reset Your Password - Flight History App"
	body := fmt.Sprintf(`
		<html>
		<body>
			<h2>Password Reset Request</h2>
			<p>Hello %s,</p>
			<p>You have requested to reset your password for your Flight History App account.</p>
			<p>Click the link below to reset your password:</p>
			<p><a href="%s" style="background-color: #007bff; color: white; padding: 10px 20px; text-decoration: none; border-radius: 5px;">Reset Password</a></p>
			<p>Or copy and paste this link into your browser:</p>
			<p>%s</p>
			<p>This link will expire in 1 hour for security reasons.</p>
			<p>If you didn't request this password reset, please ignore this email.</p>
			<br>
			<p>Best regards,<br>Flight History App Team</p>
		</body>
		</html>
	`, name, resetLink, resetLink)

	input := &ses.SendEmailInput{
		Destination: &types.Destination{
			ToAddresses: []string{toEmail},
		},
		Message: &types.Message{
			Subject: &types.Content{
				Data:    aws.String(subject),
				Charset: aws.String("UTF-8"),
			},
			Body: &types.Body{
				Html: &types.Content{
					Data:    aws.String(body),
					Charset: aws.String("UTF-8"),
				},
			},
		},
		Source: aws.String(fromEmail),
	}

	_, err := sesClient.SendEmail(context.TODO(), input)
	if err != nil {
		log.Printf("Failed to send password reset email to %s: %v", toEmail, err)
		return fmt.Errorf("failed to send password reset email: %w", err)
	}

	log.Printf("Password reset email sent successfully to %s", toEmail)
	return nil
}

// SendPasswordChangeConfirmationEmail sends a confirmation email after password change
func SendPasswordChangeConfirmationEmail(toEmail, name string) error {
	if sesClient == nil {
		return fmt.Errorf("SES client not initialized")
	}

	fromEmail := os.Getenv("SES_FROM_EMAIL")
	if fromEmail == "" {
		return fmt.Errorf("SES_FROM_EMAIL environment variable not set")
	}

	subject := "Password Changed - Flight History App"
	body := fmt.Sprintf(`
		<html>
		<body>
			<h2>Password Successfully Changed</h2>
			<p>Hello %s,</p>
			<p>Your password for your Flight History App account has been successfully changed.</p>
			<p>If you made this change, no further action is required.</p>
			<p>If you did not make this change, please contact our support team immediately.</p>
			<br>
			<p>Best regards,<br>Flight History App Team</p>
		</body>
		</html>
	`, name)

	input := &ses.SendEmailInput{
		Destination: &types.Destination{
			ToAddresses: []string{toEmail},
		},
		Message: &types.Message{
			Subject: &types.Content{
				Data:    aws.String(subject),
				Charset: aws.String("UTF-8"),
			},
			Body: &types.Body{
				Html: &types.Content{
					Data:    aws.String(body),
					Charset: aws.String("UTF-8"),
				},
			},
		},
		Source: aws.String(fromEmail),
	}

	_, err := sesClient.SendEmail(context.TODO(), input)
	if err != nil {
		log.Printf("Failed to send password change confirmation email to %s: %v", toEmail, err)
		return fmt.Errorf("failed to send password change confirmation email: %w", err)
	}

	log.Printf("Password change confirmation email sent successfully to %s", toEmail)
	return nil
}

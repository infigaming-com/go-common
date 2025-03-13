package messaging

import (
	"context"
	"fmt"

	"github.com/IBM/sarama"
	"go.uber.org/zap"
	"golang.org/x/oauth2/google"
)

type kafkaPlainAuth struct {
	username string
	password string
}

func NewKafkaPlainAuth(username, password string) *kafkaPlainAuth {
	return &kafkaPlainAuth{
		username: username,
		password: password,
	}
}

func (a *kafkaPlainAuth) Credentials() (string, string, error) {
	return a.username, a.password, nil
}

type gmkTokenProvider struct{}

func NewGmkTokenProvider(lg *zap.Logger) *gmkTokenProvider {
	return &gmkTokenProvider{}
}

func (p *gmkTokenProvider) Token() (*sarama.AccessToken, error) {
	ctx := context.Background()

	scopes := []string{
		"https://www.googleapis.com/auth/cloud-platform",
	}

	creds, err := google.FindDefaultCredentials(ctx, scopes...)
	if err != nil {
		return nil, fmt.Errorf("failed to find default credentials: %w", err)
	}

	token, err := creds.TokenSource.Token()
	if err != nil {
		return nil, fmt.Errorf("failed to get token: %w", err)
	}

	return &sarama.AccessToken{
		Token: token.AccessToken,
	}, nil
}

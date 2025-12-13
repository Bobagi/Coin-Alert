package domain

import "time"

type EmailAlert struct {
    Identifier       int64
    RecipientAddress string
    Subject          string
    MessageBody      string
    CreatedAt        time.Time
}

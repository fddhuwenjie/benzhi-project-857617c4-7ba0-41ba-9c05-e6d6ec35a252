package application

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"time"
)

type Clock interface {
	Now() time.Time
}

type SystemClock struct{}

func (SystemClock) Now() time.Time { return time.Now().UTC() }

type IDGenerator interface {
	New(prefix string) string
}

type RandomIDGenerator struct{}

func (RandomIDGenerator) New(prefix string) string {
	var bytes [10]byte
	if _, err := rand.Read(bytes[:]); err != nil {
		return fmt.Sprintf("%s-%d", prefix, time.Now().UnixNano())
	}
	return prefix + "-" + hex.EncodeToString(bytes[:])
}

type FixedClock struct {
	Value time.Time
}

func (c FixedClock) Now() time.Time { return c.Value.UTC() }

type SequenceIDGenerator struct {
	Values []string
	index  int
}

func (g *SequenceIDGenerator) New(prefix string) string {
	if g.index < len(g.Values) {
		value := g.Values[g.index]
		g.index++
		return value
	}
	g.index++
	return fmt.Sprintf("%s-%d", prefix, g.index)
}

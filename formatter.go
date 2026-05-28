package logs

type LoggerFormatter interface {
	Format(e Entry) []byte
}

type HumanLogFormatter struct{}

func (HumanLogFormatter) Format(e Entry) []byte {
	return []byte(formatHuman(e))
}

type GCPLogFormatter struct{}

func (GCPLogFormatter) Format(e Entry) []byte {
	return []byte(formatJSON(e))
}

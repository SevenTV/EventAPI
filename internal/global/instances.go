package global

import "github.com/seventv/eventapi/internal/instance"

type Instances struct {
	Redis            instance.Redis
	Monitoring       instance.Monitoring
	ConcurrencyValue int32
}

package global

import "github.com/SevenTV/EventAPI/src/instance"

type Instances struct {
	Redis      instance.Redis
	Monitoring instance.Monitoring
}

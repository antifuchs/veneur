package metricingester

import (
	"github.com/stripe/veneur/samplers"
)

type aggWorker struct {
	samplers samplerEnvelope

	inC    chan Metric
	mergeC chan Digest
	flush  chan chan<- samplerEnvelope
}

func (a aggWorker) Start() {
	go func() {
		for {
			select {
			case m, ok := <-a.inC:
				if !ok {
					return
				}
				a.ingest(m)
			case d, ok := <-a.mergeC:
				if !ok {
					return
				}
				a.merge(d)
			case responseCh := <-a.flush:
				responseCh <- a.samplers
				a.samplers = newSamplerEnvelope()
			}
		}
	}()
}

func (a aggWorker) Ingest(m Metric) {
	a.inC <- m
}

func (a aggWorker) Merge(d Digest) {
	a.mergeC <- d
}

func (a aggWorker) Stop() {
	close(a.inC)
	close(a.flush)
}

func (a aggWorker) Flush() samplerEnvelope {
	rcv := make(chan samplerEnvelope)
	a.flush <- rcv
	return <-rcv
}

func (a aggWorker) ingest(m Metric) {
	key := m.Key()
	switch m.metricType {
	case counter:
		if _, present := a.samplers.counters[key]; !present {
			a.samplers.counters[key] = samplers.NewCounter(m.name, m.tags)
		}
		a.samplers.counters[key].Sample(float64(m.countervalue), m.samplerate)
	case gauge:
		if _, present := a.samplers.gauges[key]; !present {
			a.samplers.gauges[key] = samplers.NewGauge(m.name, m.tags)
		}
		a.samplers.gauges[key].Sample(m.gaugevalue, m.samplerate)
	case set:
		if _, present := a.samplers.sets[key]; !present {
			a.samplers.sets[key] = samplers.NewSet(m.name, m.tags)
		}
		a.samplers.sets[key].Sample(m.setvalue, m.samplerate)
	case histogram:
		if _, present := a.samplers.histograms[key]; !present {
			a.samplers.histograms[key] = samplers.NewHist(m.name, m.tags)
		}
		a.samplers.histograms[key].Sample(m.histovalue, m.samplerate)
	}
}

func (a aggWorker) merge(d Digest) {
	key := d.Key()
	switch d.digestType {
	case histoDigest:
		if _, present := a.samplers.histograms[key]; !present {
			a.samplers.histograms[key] = samplers.NewHist(d.name, d.tags)
		}
		a.samplers.histograms[key].Merge(d.histodigest)
	case setDigest:
		if _, present := a.samplers.sets[key]; !present {
			a.samplers.sets[key] = samplers.NewSet(d.name, d.tags)
		}
		a.samplers.sets[key].Merge(d.setdigest)
	}
}
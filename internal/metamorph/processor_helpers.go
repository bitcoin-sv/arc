package metamorph

func (p *Processor) GetStats(debugItems bool) *ProcessorStats {
	if debugItems {
		p.ProcessorResponseMap.logMapItems(p.logger)
	}

	return &ProcessorStats{
		ChannelMapSize: int32(p.ProcessorResponseMap.Len()),
	}
}

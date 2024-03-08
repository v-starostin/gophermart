package currency

func ConvertToPrimary(v int64) float64 {
	return float64(v) / 1000
}

func ConvertToSubunit(v float64) int64 {
	return int64(v * 1000)
}

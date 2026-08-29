package cache

// SpeciesIndexWriteCountForTest reports how many times StoreSpeciesIndex ran.
func (s *Store) SpeciesIndexWriteCountForTest() uint32 {
	if s == nil {
		return 0
	}
	return s.speciesIndexWrites.Load()
}

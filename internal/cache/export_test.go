package cache

// SpeciesIndexWriteCountForTest reports how many times StoreSpeciesIndex ran.
func (s *Store) SpeciesIndexWriteCountForTest() uint32 {
	if s == nil {
		return 0
	}
	return s.speciesIndexWrites.Load()
}

// InvalidateV3DatasetForTest drops the in-memory v3 cache so tests can observe disk reload.
func (s *Store) InvalidateV3DatasetForTest() {
	if s == nil {
		return
	}
	s.invalidateV3Dataset()
}

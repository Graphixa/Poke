package cache

// SpeciesIndexWriteCountForTest reports how many times StoreSpeciesIndex ran.
func (s *Store) SpeciesIndexWriteCountForTest() uint32 {
	if s == nil {
		return 0
	}
	return s.speciesIndexWrites.Load()
}

// InvalidateLoadedSnapshotForTest drops the in-memory snapshot cache so tests can observe disk reload.
func (s *Store) InvalidateLoadedSnapshotForTest() {
	if s == nil {
		return
	}
	s.invalidateLoadedSnapshot()
}

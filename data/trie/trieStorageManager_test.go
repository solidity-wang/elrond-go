package trie

import (
	"io/ioutil"
	"os"
	"path"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/ElrondNetwork/elrond-go/config"
	"github.com/ElrondNetwork/elrond-go/data"
	"github.com/ElrondNetwork/elrond-go/data/mock"
	"github.com/ElrondNetwork/elrond-go/storage/storageUnit"
	"github.com/stretchr/testify/assert"
)

func TestNewTrieStorageManagerNilDb(t *testing.T) {
	t.Parallel()

	ts, err := NewTrieStorageManager(nil, &config.DBConfig{}, &mock.EvictionWaitingList{})
	assert.Nil(t, ts)
	assert.Equal(t, ErrNilDatabase, err)
}

func TestNewTrieStorageManagerNilSnapshotDbConfig(t *testing.T) {
	t.Parallel()

	ts, err := NewTrieStorageManager(mock.NewMemDbMock(), nil, &mock.EvictionWaitingList{})
	assert.Nil(t, ts)
	assert.Equal(t, ErrNilSnapshotDbConfig, err)
}

func TestNewTrieStorageManagerNilEwl(t *testing.T) {
	t.Parallel()

	ts, err := NewTrieStorageManager(mock.NewMemDbMock(), &config.DBConfig{}, nil)
	assert.Nil(t, ts)
	assert.Equal(t, ErrNilEvictionWaitingList, err)
}

func TestNewTrieStorageManagerOkVals(t *testing.T) {
	t.Parallel()

	ts, err := NewTrieStorageManager(mock.NewMemDbMock(), &config.DBConfig{}, &mock.EvictionWaitingList{})
	assert.Nil(t, err)
	assert.NotNil(t, ts)
}

func TestTrieStorageManager_Clone(t *testing.T) {
	t.Parallel()

	ts, _ := NewTrieStorageManager(mock.NewMemDbMock(), &config.DBConfig{}, &mock.EvictionWaitingList{})

	newTs := ts.Clone()
	newTs, _ = newTs.(*trieStorageManager)
	assert.True(t, ts != newTs)
}

func TestTrieDatabasePruning(t *testing.T) {
	t.Parallel()

	db := mock.NewMemDbMock()
	msh, hsh := getTestMarshAndHasher()
	size := uint(1)
	evictionWaitList, _ := mock.NewEvictionWaitingList(size, mock.NewMemDbMock(), msh)
	trieStorage, _ := NewTrieStorageManager(db, &config.DBConfig{}, evictionWaitList)

	tr := &patriciaMerkleTrie{
		trieStorage: trieStorage,
		oldHashes:   make([][]byte, 0),
		oldRoot:     make([]byte, 0),
		marshalizer: msh,
		hasher:      hsh,
	}

	_ = tr.Update([]byte("doe"), []byte("reindeer"))
	_ = tr.Update([]byte("dog"), []byte("puppy"))
	_ = tr.Update([]byte("dogglesworth"), []byte("cat"))
	_ = tr.Commit()

	key := []byte{6, 4, 6, 15, 6, 7, 16}
	oldHashes := make([][]byte, 0)
	n := tr.root
	rootHash, _ := tr.Root()
	oldHashes = append(oldHashes, rootHash)

	for i := 0; i < 3; i++ {
		n, key, _ = n.getNext(key, db)
		oldHashes = append(oldHashes, n.getHash())
	}

	_ = tr.Update([]byte("dog"), []byte("doee"))
	_ = tr.Commit()

	err := tr.Prune(rootHash, data.OldRoot)
	assert.Nil(t, err)

	for i := range oldHashes {
		encNode, err := tr.Database().Get(oldHashes[i])
		assert.Nil(t, encNode)
		assert.NotNil(t, err)
	}
}

func TestRecreateTrieFromSnapshotDb(t *testing.T) {
	t.Parallel()

	tr, trieStorage, _ := newEmptyTrie()
	_ = tr.Update([]byte("doe"), []byte("reindeer"))
	_ = tr.Update([]byte("dog"), []byte("puppy"))
	_ = tr.Update([]byte("dogglesworth"), []byte("cat"))

	_ = tr.Commit()
	rootHash, _ := tr.Root()
	tr.TakeSnapshot(rootHash)

	for trieStorage.snapshotsBuffer.len() != 0 {
		time.Sleep(snapshotDelay)
	}

	_ = tr.Update([]byte("doge"), []byte("doge"))
	_ = tr.Commit()
	_ = tr.Prune(rootHash, data.OldRoot)

	val, err := tr.Database().Get(rootHash)
	assert.Nil(t, val)
	assert.NotNil(t, err)

	newTrie, err := tr.Recreate(rootHash)
	assert.Nil(t, err)
	assert.NotNil(t, newTrie)
}

func TestEachSnapshotCreatesOwnDatabase(t *testing.T) {
	t.Parallel()

	testVals := []struct {
		key   []byte
		value []byte
	}{
		{[]byte("doe"), []byte("reindeer")},
		{[]byte("dog"), []byte("puppy")},
		{[]byte("dogglesworth"), []byte("cat")},
	}

	tr, trieStorage, _ := newEmptyTrie()

	for _, testVal := range testVals {
		_ = tr.Update(testVal.key, testVal.value)
		_ = tr.Commit()
		tr.TakeSnapshot(tr.root.getHash())
		for trieStorage.snapshotsBuffer.len() != 0 {
			time.Sleep(snapshotDelay)
		}

		snapshotId := strconv.Itoa(trieStorage.snapshotId - 1)
		snapshotPath := path.Join(trieStorage.snapshotDbCfg.FilePath, snapshotId)
		f, _ := os.Stat(snapshotPath)
		assert.True(t, f.IsDir())
	}

	assert.Equal(t, len(testVals), trieStorage.snapshotId)
}

func TestDeleteOldSnapshots(t *testing.T) {
	t.Parallel()

	testVals := []struct {
		key   []byte
		value []byte
	}{
		{[]byte("doe"), []byte("reindeer")},
		{[]byte("dog"), []byte("puppy")},
		{[]byte("dogglesworth"), []byte("cat")},
		{[]byte("horse"), []byte("mustang")},
	}

	tr, trieStorage, _ := newEmptyTrie()

	for _, testVal := range testVals {
		_ = tr.Update(testVal.key, testVal.value)
		_ = tr.Commit()
		tr.TakeSnapshot(tr.root.getHash())
		for trieStorage.snapshotsBuffer.len() != 0 {
			time.Sleep(snapshotDelay)
		}
	}

	snapshots, _ := ioutil.ReadDir(trieStorage.snapshotDbCfg.FilePath)
	assert.Equal(t, 2, len(snapshots))
	assert.Equal(t, "2", snapshots[0].Name())
	assert.Equal(t, "3", snapshots[1].Name())
}

func TestPruningIsBufferedWhileSnapshoting(t *testing.T) {
	t.Parallel()

	nrVals := 100000
	index := 0
	var rootHashes [][]byte

	db := mock.NewMemDbMock()
	msh, hsh := getTestMarshAndHasher()
	evictionWaitListSize := uint(100)
	evictionWaitList, _ := mock.NewEvictionWaitingList(evictionWaitListSize, mock.NewMemDbMock(), msh)

	tempDir, _ := ioutil.TempDir("", "leveldb_temp")
	cfg := &config.DBConfig{
		FilePath:          tempDir,
		Type:              string(storageUnit.LvlDbSerial),
		BatchDelaySeconds: 1,
		MaxBatchSize:      40000,
		MaxOpenFiles:      10,
	}
	trieStorage, _ := NewTrieStorageManager(db, cfg, evictionWaitList)

	tr := &patriciaMerkleTrie{
		trieStorage: trieStorage,
		marshalizer: msh,
		hasher:      hsh,
	}

	for i := 0; i < nrVals; i++ {
		_ = tr.Update(tr.hasher.Compute(strconv.Itoa(index)), tr.hasher.Compute(strconv.Itoa(index)))
		index++
	}

	_ = tr.Commit()
	rootHash := tr.root.getHash()
	rootHashes = append(rootHashes, rootHash)
	tr.TakeSnapshot(rootHash)

	nrRounds := 10
	nrUpdates := 1000
	for i := 0; i < nrRounds; i++ {
		for j := 0; j < nrUpdates; j++ {
			_ = tr.Update(tr.hasher.Compute(strconv.Itoa(index)), tr.hasher.Compute(strconv.Itoa(index)))
			index++
		}
		_ = tr.Commit()

		previousRootHashIndex := len(rootHashes) - 1
		currentRootHash := tr.root.getHash()

		_ = tr.Prune(rootHashes[previousRootHashIndex], data.OldRoot)
		_ = tr.Prune(currentRootHash, data.NewRoot)
		rootHashes = append(rootHashes, currentRootHash)
	}
	numKeysToBeEvicted := 21
	assert.Equal(t, numKeysToBeEvicted, len(evictionWaitList.Cache))
	assert.NotEqual(t, 0, trieStorage.pruningBufferLength())

	for trieStorage.snapshotsBuffer.len() != 0 {
		time.Sleep(snapshotDelay)
	}
	time.Sleep(snapshotDelay)

	for i := range rootHashes {
		val, err := tr.Database().Get(rootHashes[i])
		assert.Nil(t, val)
		assert.NotNil(t, err)
	}

	time.Sleep(batchDelay)
	val, err := trieStorage.snapshots[0].Get(rootHash)
	assert.NotNil(t, val)
	assert.Nil(t, err)
}

func (tsm *trieStorageManager) pruningBufferLength() int {
	tsm.storageOperationMutex.Lock()
	defer tsm.storageOperationMutex.Unlock()

	return len(tsm.pruningBuffer)
}

func TestTrieCheckpoint(t *testing.T) {
	t.Parallel()

	tr, trieStorage, _ := newEmptyTrie()
	_ = tr.Update([]byte("doe"), []byte("reindeer"))
	_ = tr.Update([]byte("dog"), []byte("puppy"))
	_ = tr.Update([]byte("dogglesworth"), []byte("cat"))

	_ = tr.Commit()
	tr.TakeSnapshot(tr.root.getHash())

	for trieStorage.snapshotsBuffer.len() != 0 {
		time.Sleep(snapshotDelay)
	}

	_ = tr.Update([]byte("doge"), []byte("reindeer"))
	_ = tr.Commit()

	val, err := tr.Get([]byte("doge"))
	assert.Nil(t, err)
	assert.Equal(t, []byte("reindeer"), val)

	snapshotTrieStorage, _ := NewTrieStorageManager(trieStorage.snapshots[0], &config.DBConfig{}, &mock.EvictionWaitingList{})
	collapsedRoot, _ := tr.root.getCollapsed()
	snapshotTrie := &patriciaMerkleTrie{
		root:        collapsedRoot,
		trieStorage: snapshotTrieStorage,
		marshalizer: tr.marshalizer,
		hasher:      tr.hasher,
	}

	val, err = snapshotTrie.Get([]byte("doge"))
	assert.NotNil(t, err)
	assert.Nil(t, val)

	tr.SetCheckpoint(tr.root.getHash())

	for trieStorage.snapshotsBuffer.len() != 0 {
		time.Sleep(snapshotDelay)
	}

	val, err = snapshotTrie.Get([]byte("doge"))
	assert.Nil(t, err)
	assert.Equal(t, []byte("reindeer"), val)
}

func TestTrieCheckpointWithNoSnapshotCreatesSnapshot(t *testing.T) {
	t.Parallel()

	tr, trieStorage, _ := newEmptyTrie()
	_ = tr.Update([]byte("doe"), []byte("reindeer"))
	_ = tr.Update([]byte("dog"), []byte("puppy"))
	_ = tr.Update([]byte("dogglesworth"), []byte("cat"))

	assert.Equal(t, 0, len(trieStorage.snapshots))

	_ = tr.Commit()
	tr.SetCheckpoint(tr.root.getHash())

	for trieStorage.snapshotsBuffer.len() != 0 {
		time.Sleep(snapshotDelay)
	}

	assert.Equal(t, 1, len(trieStorage.snapshots))
}

func TestTrieSnapshottingAndCheckpointConcurrently(t *testing.T) {
	t.Parallel()

	tr, trieStorage, _ := newEmptyTrie()
	_ = tr.Update([]byte("doe"), []byte("reindeer"))
	_ = tr.Update([]byte("dog"), []byte("puppy"))
	_ = tr.Update([]byte("dogglesworth"), []byte("cat"))
	_ = tr.Commit()

	tr.TakeSnapshot(tr.root.getHash())
	for trieStorage.snapshotsBuffer.len() != 0 {
		time.Sleep(time.Second)
	}

	numSnapshots := 10
	numCheckpoints := 10
	totalNumSnapshot := numSnapshots + 1

	var snapshotWg sync.WaitGroup
	var checkpointWg sync.WaitGroup
	snapshotWg.Add(numSnapshots)
	checkpointWg.Add(numCheckpoints)

	for i := 0; i < numSnapshots; i++ {
		go func() {
			rootHash, _ := tr.Root()
			tr.TakeSnapshot(rootHash)
			snapshotWg.Done()
		}()
	}

	for i := 0; i < numCheckpoints; i++ {
		go func() {
			rootHash, _ := tr.Root()
			tr.SetCheckpoint(rootHash)
			checkpointWg.Done()
		}()
	}

	snapshotWg.Wait()
	checkpointWg.Wait()

	for trieStorage.snapshotsBuffer.len() != 0 {
		time.Sleep(time.Second)
	}

	assert.Equal(t, totalNumSnapshot, trieStorage.snapshotId)

	lastSnapshot := len(trieStorage.snapshots) - 1
	val, err := trieStorage.snapshots[lastSnapshot].Get(tr.root.getHash())
	assert.NotNil(t, val)
	assert.Nil(t, err)
}

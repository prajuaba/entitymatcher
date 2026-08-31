package matcher

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

func TestMongoPagingIsStableUnderConcurrentUpdate(t *testing.T) {
	mongoURI := os.Getenv("TEST_MONGO_URI")
	if mongoURI == "" {
		t.Skip("TEST_MONGO_URI not set")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	u, err := url.Parse(mongoURI)
	require.NoError(t, err, "failed to parse TEST_MONGO_URI")

	host := u.Hostname()
	portStr := u.Port()
	port, _ := strconv.Atoi(portStr)

	cfg := ConnectionConfig{
		Type:         SourceTypeMongoDB,
		Host:         host,
		Port:         port,
		Database:     "emtest",
		TableOrQuery: "page_probe",
	}

	observerURI := fmt.Sprintf("mongodb://%s:%d", host, port)
	observerClient, err := mongo.Connect(ctx, options.Client().ApplyURI(observerURI))
	require.NoError(t, err, "failed to connect observer client")
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = observerClient.Disconnect(ctx)
	}()

	coll := observerClient.Database("emtest").Collection("page_probe")
	err = coll.Drop(ctx)
	if err != nil && !strings.Contains(err.Error(), "ns doesn't exist") {
		require.NoError(t, err, "failed to drop page_probe collection")
	}

	// Insert 40 documents
	docs := make([]interface{}, 40)
	for i := 1; i <= 40; i++ {
		docs[i-1] = bson.M{"seq": i, "name": fmt.Sprintf("n%d", i)}
	}
	_, err = coll.InsertMany(ctx, docs)
	require.NoError(t, err, "failed to insert documents")

	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		err = coll.Drop(ctx)
		if err != nil && !strings.Contains(err.Error(), "ns doesn't exist") {
			require.NoError(t, err, "failed to drop page_probe collection")
		}
	}()

	conn, err := NewDataConnector(cfg)
	require.NoError(t, err, "NewDataConnector failed")
	defer conn.Close()

	// Fetch page 1
	page1, err := conn.FetchRecords(ctx, 10, 0)
	require.NoError(t, err, "FetchRecords page 1 failed")

	var seqs1 []int64
	for _, row := range page1 {
		seqs1 = append(seqs1, toInt64(t, row["seq"]))
	}

	// WiredTiger orders unsorted natural reads by RecordId, an internal
	// storage-slot identifier that is independent of any document field.
	// Growing a document via $set does not change its RecordId, so a padding
	// UpdateMany leaves natural order untouched and such a test would pass
	// whether or not the connector sorts. Deleting a document and
	// reinserting it assigns a brand new RecordId, physically relocating it
	// to the end of natural order, which is what actually distinguishes a
	// sorted fetch from an unsorted one.
	//
	// The reinsert deliberately preserves each document's original "_id"
	// value (the default sort field used by FetchRecords) instead of
	// letting the driver mint a fresh one: FetchRecords sorts by "_id", and
	// a fresh ObjectID would be strictly greater than every existing one,
	// which would shift the sort order too and make a correctly-sorted
	// fetch fail this test for reasons unrelated to whether SetSort is
	// applied. Keeping "_id" stable while only RecordId moves isolates the
	// natural-order churn this test is meant to detect.
	churnFilter := bson.M{"seq": bson.M{"$lte": 10}}
	cursor, err := coll.Find(ctx, churnFilter)
	require.NoError(t, err, "failed to find documents to churn")
	var docsToMove []bson.M
	require.NoError(t, cursor.All(ctx, &docsToMove), "failed to decode documents to churn")
	cursor.Close(ctx)

	_, err = coll.DeleteMany(ctx, churnFilter)
	require.NoError(t, err, "failed to delete churned documents")

	for i, doc := range docsToMove {
		_, err = coll.InsertOne(ctx, doc)
		require.NoError(t, err, "failed to reinsert churned document at index %d", i)
	}

	// Fetch pages 2, 3, 4
	page2, err := conn.FetchRecords(ctx, 10, 10)
	require.NoError(t, err, "FetchRecords page 2 failed")
	page3, err := conn.FetchRecords(ctx, 10, 20)
	require.NoError(t, err, "FetchRecords page 3 failed")
	page4, err := conn.FetchRecords(ctx, 10, 30)
	require.NoError(t, err, "FetchRecords page 4 failed")

	// Collect all seqs
	allSeqs := make(map[int64]int)
	for _, row := range page1 {
		seq := toInt64(t, row["seq"])
		allSeqs[seq]++
	}
	for _, row := range page2 {
		seq := toInt64(t, row["seq"])
		allSeqs[seq]++
	}
	for _, row := range page3 {
		seq := toInt64(t, row["seq"])
		allSeqs[seq]++
	}
	for _, row := range page4 {
		seq := toInt64(t, row["seq"])
		allSeqs[seq]++
	}

	// Verify no duplicates or missing seqs
	require.Len(t, allSeqs, 40, "expected exactly 40 distinct seq values, got %d", len(allSeqs))

	for i := 1; i <= 40; i++ {
		_, exists := allSeqs[int64(i)]
		require.True(t, exists, "seq %d is missing", i)
		require.Equal(t, 1, allSeqs[int64(i)], "seq %d appears %d times, expected 1", i, allSeqs[int64(i)])
	}
}

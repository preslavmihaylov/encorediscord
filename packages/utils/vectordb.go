package utils

import (
	"context"
	"fmt"

	"github.com/pinecone-io/go-pinecone/pinecone"
)

func ConnectToVectorDBIndex(
	ctx context.Context, pineconeClient *pinecone.Client, indexName string,
) (*pinecone.IndexConnection, error) {
	index, err := createOrGetVectorDBIndex(ctx, pineconeClient, indexName)
	if err != nil {
		return nil, err
	}

	indexConn, err := pineconeClient.Index(index.Host)
	if err != nil {
		return nil, fmt.Errorf("couldn't connect to index: %w", err)
	}

	return indexConn, nil
}

func createOrGetVectorDBIndex(
	ctx context.Context, pineconeClient *pinecone.Client, indexName string,
) (*pinecone.Index, error) {
	indices, err := pineconeClient.ListIndexes(ctx)
	if err != nil {
		panic("Error listing indexes: " + err.Error())
	}

	for _, index := range indices {
		if index.Name == indexName {
			return index, nil
		}
	}

	index, err := pineconeClient.CreateServerlessIndex(ctx, &pinecone.CreateServerlessIndexRequest{
		Name:      indexName,
		Dimension: 3072,
		Metric:    "cosine",
		Cloud:     "aws",
		// this is the only region supported for the default tier
		Region: "us-east-1",
	})
	if err != nil {
		return nil, fmt.Errorf("couldn't create index: %w", err)
	}

	return index, nil
}

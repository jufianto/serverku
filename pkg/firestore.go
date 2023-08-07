package pkg

import (
	"context"
	"log"

	firestore "cloud.google.com/go/firestore"
)

func SetupFirestoreClient(ctx context.Context) *firestore.Client {
	// Sets your Google Cloud Platform project ID.
	projectID := "YOUR_PROJECT_ID"

	client, err := firestore.NewClient(ctx, projectID)
	if err != nil {
		log.Fatalf("Failed to create client: %v", err)
	}
	// Close client when done with
	// defer client.Close()
	return client
}

package controller

import (
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"errors"
	"net/http"
	"strings"

	"github.com/QuantumNous/new-api/model"
	"github.com/gin-gonic/gin"
)

// New worker credentials are random, are sent only during successful pairing,
// and are stored as hashes in the database. They are never accepted in URLs.
func newByogWorkerCredential() (string, string, error) {
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return "", "", err
	}
	token := hex.EncodeToString(raw)
	digest := sha256.Sum256([]byte(token))
	return token, hex.EncodeToString(digest[:]), nil
}

func authenticatedByogWorker(c *gin.Context, nodeID string) (*model.ByogNode, error) {
	auth := c.GetHeader("Authorization")
	if !strings.HasPrefix(auth, "Bearer ") || len(auth) != len("Bearer ")+64 || nodeID == "" {
		return nil, errors.New("invalid worker credential")
	}
	token := strings.TrimPrefix(auth, "Bearer ")
	if _, err := hex.DecodeString(token); err != nil {
		return nil, errors.New("invalid worker credential")
	}
	node, err := model.GetByogNodeByNodeId(nodeID)
	if err != nil || node == nil || node.UserId <= 0 || node.Status == "banned" || len(node.WorkerSecretHash) != 64 {
		return nil, errors.New("invalid worker credential")
	}
	expected, err := hex.DecodeString(node.WorkerSecretHash)
	if err != nil {
		return nil, errors.New("invalid worker credential")
	}
	digest := sha256.Sum256([]byte(token))
	if subtle.ConstantTimeCompare(expected, digest[:]) != 1 {
		return nil, errors.New("invalid worker credential")
	}
	return node, nil
}

func requireByogWorker(c *gin.Context, nodeID string) (*model.ByogNode, bool) {
	if ownerAccess, _ := c.Get("byog_owner_access"); ownerAccess == true {
		node, err := model.GetByogNodeByNodeId(nodeID)
		if err == nil && node != nil && node.UserId > 0 && node.UserId == c.GetInt("id") {
			return node, true
		}
		c.JSON(http.StatusUnauthorized, gin.H{"success": false, "message": "Invalid node owner"})
		return nil, false
	}
	node, err := authenticatedByogWorker(c, nodeID)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"success": false, "message": "Invalid worker credentials"})
		return nil, false
	}
	return node, true
}

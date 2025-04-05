package utils

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewUserDataStore(t *testing.T) {
	store := NewUserDataStore()

	assert.NotNil(t, store, "Store should not be nil")
	assert.NotNil(t, store.userData, "Store's userData map should not be nil")
	assert.Empty(t, store.userData, "Store should start empty")
}

func TestUserDataStore_Get_NonExistentUser(t *testing.T) {
	store := NewUserDataStore()
	var userID int64 = 12345

	value, exists := store.Get(userID, "testKey")

	assert.False(t, exists, "Get for non-existent user should return false")
	assert.Nil(t, value, "Get for non-existent user should return nil value")
}

func TestUserDataStore_Get_NonExistentKey(t *testing.T) {
	store := NewUserDataStore()
	var userID int64 = 12345

	// Add user but not the key we'll test
	store.Set(userID, "otherKey", "otherValue")

	value, exists := store.Get(userID, "testKey")

	assert.False(t, exists, "Get for non-existent key should return false")
	assert.Nil(t, value, "Get for non-existent key should return nil value")
}

func TestUserDataStore_Get_ExistingKeyValue(t *testing.T) {
	store := NewUserDataStore()
	var userID int64 = 12345
	expectedValue := "testValue"

	store.Set(userID, "testKey", expectedValue)

	value, exists := store.Get(userID, "testKey")

	assert.True(t, exists, "Get for existing key should return true")
	assert.Equal(t, expectedValue, value, "Get should return the correct value")
}

func TestUserDataStore_Set_NewUser(t *testing.T) {
	store := NewUserDataStore()
	var userID int64 = 12345
	expectedValue := "testValue"

	store.Set(userID, "testKey", expectedValue)

	// Verify internal state directly
	assert.Contains(t, store.userData, userID, "User should be added to the store")
	assert.Contains(t, store.userData[userID], "testKey", "Key should be added for the user")
	assert.Equal(t, expectedValue, store.userData[userID]["testKey"], "Value should be stored correctly")

	// Verify using Get method
	value, exists := store.Get(userID, "testKey")
	assert.True(t, exists)
	assert.Equal(t, expectedValue, value)
}

func TestUserDataStore_Set_ExistingUser(t *testing.T) {
	store := NewUserDataStore()
	var userID int64 = 12345

	// Set an initial value
	store.Set(userID, "testKey", "initialValue")

	// Update the value
	expectedValue := "updatedValue"
	store.Set(userID, "testKey", expectedValue)

	// Verify value was updated
	value, exists := store.Get(userID, "testKey")
	assert.True(t, exists)
	assert.Equal(t, expectedValue, value, "Value should be updated")
}

func TestUserDataStore_Set_MultipleKeysAndUsers(t *testing.T) {
	store := NewUserDataStore()
	var user1ID int64 = 12345
	var user2ID int64 = 67890

	// Add multiple keys for multiple users
	store.Set(user1ID, "key1", "value1-1")
	store.Set(user1ID, "key2", "value1-2")
	store.Set(user2ID, "key1", "value2-1")

	// Verify all values
	val1, exists := store.Get(user1ID, "key1")
	assert.True(t, exists)
	assert.Equal(t, "value1-1", val1)

	val2, exists := store.Get(user1ID, "key2")
	assert.True(t, exists)
	assert.Equal(t, "value1-2", val2)

	val3, exists := store.Get(user2ID, "key1")
	assert.True(t, exists)
	assert.Equal(t, "value2-1", val3)
}

func TestUserDataStore_Clear(t *testing.T) {
	store := NewUserDataStore()
	var user1ID int64 = 12345
	var user2ID int64 = 67890

	// Add data for two users
	store.Set(user1ID, "key1", "value1")
	store.Set(user2ID, "key1", "value2")

	// Clear one user
	store.Clear(user1ID)

	// Check user1 data is gone
	_, exists := store.Get(user1ID, "key1")
	assert.False(t, exists, "Data for cleared user should not exist")

	// Check user2 data still exists
	value, exists := store.Get(user2ID, "key1")
	assert.True(t, exists, "Data for other users should remain intact")
	assert.Equal(t, "value2", value)

	// Verify internal state
	assert.NotContains(t, store.userData, user1ID, "Cleared user should be removed from userData map")
	assert.Contains(t, store.userData, user2ID, "Other users should remain in userData map")
}

func TestUserDataStore_DifferentValueTypes(t *testing.T) {
	store := NewUserDataStore()
	var userID int64 = 12345

	// Test with different value types
	stringValue := "string value"
	intValue := 42
	boolValue := true
	sliceValue := []string{"a", "b", "c"}
	mapValue := map[string]int{"a": 1, "b": 2}

	// Set different types
	store.Set(userID, "string", stringValue)
	store.Set(userID, "int", intValue)
	store.Set(userID, "bool", boolValue)
	store.Set(userID, "slice", sliceValue)
	store.Set(userID, "map", mapValue)

	// Verify stored correctly
	val1, exists := store.Get(userID, "string")
	assert.True(t, exists)
	assert.Equal(t, stringValue, val1)

	val2, exists := store.Get(userID, "int")
	assert.True(t, exists)
	assert.Equal(t, intValue, val2)

	val3, exists := store.Get(userID, "bool")
	assert.True(t, exists)
	assert.Equal(t, boolValue, val3)

	val4, exists := store.Get(userID, "slice")
	assert.True(t, exists)
	assert.Equal(t, sliceValue, val4)

	val5, exists := store.Get(userID, "map")
	assert.True(t, exists)
	assert.Equal(t, mapValue, val5)
}

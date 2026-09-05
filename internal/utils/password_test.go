package utils

import (
	"testing"
)

func TestHashPassword(t *testing.T) {
	tests := []struct {
		name     string
		password string
		wantErr  bool
	}{
		{
			name:     "valid password",
			password: "test123",
			wantErr:  false,
		},
		{
			name:     "empty password",
			password: "",
			wantErr:  false,
		},
		{
			name:     "long password",
			password: "verylongpassword1234567890!@#$%^&*()",
			wantErr:  false,
		},
		{
			name:     "password with special chars",
			password: "p@ssw0rd!@#$%^&*()_+",
			wantErr:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			hash, err := HashPassword(tt.password)
			if (err != nil) != tt.wantErr {
				t.Errorf("HashPassword() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && len(hash) == 0 {
				t.Error("HashPassword() returned empty hash")
			}
		})
	}
}

func TestCheckPasswordHash(t *testing.T) {
	password := "test123"
	hash, err := HashPassword(password)
	if err != nil {
		t.Fatalf("Failed to hash password: %v", err)
	}

	tests := []struct {
		name     string
		password string
		hash     string
		expected bool
	}{
		{
			name:     "correct password",
			password: password,
			hash:     string(hash),
			expected: true,
		},
		{
			name:     "wrong password",
			password: "wrongpass",
			hash:     string(hash),
			expected: false,
		},
		{
			name:     "empty password",
			password: "",
			hash:     string(hash),
			expected: false,
		},
		{
			name:     "empty hash",
			password: password,
			hash:     "",
			expected: false,
		},
		{
			name:     "invalid hash",
			password: password,
			hash:     "invalidhash",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := CheckPasswordHash(tt.password, tt.hash)
			if result != tt.expected {
				t.Errorf("CheckPasswordHash() = %v, expected %v", result, tt.expected)
			}
		})
	}
}

func TestHashPassword_Uniqueness(t *testing.T) {
	password := "test123"

	hash1, err := HashPassword(password)
	if err != nil {
		t.Fatalf("Failed to hash password: %v", err)
	}

	hash2, err := HashPassword(password)
	if err != nil {
		t.Fatalf("Failed to hash password: %v", err)
	}

	// Из-за соли хеши должны быть разными
	if string(hash1) == string(hash2) {
		t.Error("HashPassword() generated same hash for same password (salt may not be working)")
	}

	// Но проверка должна проходить для обоих
	if !CheckPasswordHash(password, string(hash1)) {
		t.Error("CheckPasswordHash() failed for hash1")
	}
	if !CheckPasswordHash(password, string(hash2)) {
		t.Error("CheckPasswordHash() failed for hash2")
	}
}

func BenchmarkHashPassword(b *testing.B) {
	password := "test123"
	for i := 0; i < b.N; i++ {
		HashPassword(password)
	}
}

func BenchmarkCheckPasswordHash(b *testing.B) {
	password := "test123"
	hash, _ := HashPassword(password)
	for i := 0; i < b.N; i++ {
		CheckPasswordHash(password, string(hash))
	}
}

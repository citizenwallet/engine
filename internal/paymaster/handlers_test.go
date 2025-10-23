package paymaster

import (
	"encoding/hex"
	"fmt"
	"math/big"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
)

const (
	// Solidity contract offsets for parsing PaymasterAndData
	VALID_TIMESTAMP_OFFSET = 20 // Start of validity period (after paymaster address)
	SIGNATURE_OFFSET       = 84 // Start of signature (after address + validity)
)

func TestPaymasterAndDataEncoding(t *testing.T) {
	// This test verifies the PaymasterAndData structure matches the Solidity parsing:
	// VALID_TIMESTAMP_OFFSET = 20 (after paymaster address)
	// SIGNATURE_OFFSET = 84 (after address + ABI-encoded validity)
	// Structure: [address(20 bytes)][validity(64 bytes ABI-encoded)][signature(65 bytes)]

	// Create test data
	paymasterAddr := common.HexToAddress("0x1234567890123456789012345678901234567890")

	// Create validity period (uint48 values)
	now := time.Now().Unix()
	validUntil := big.NewInt(now + 300) // 5 minutes from now
	validAfter := big.NewInt(now - 10)  // 10 seconds ago

	// Ensure the values fit within 48 bits
	if validUntil.BitLen() > 48 {
		t.Fatalf("validUntil exceeds 48 bits: %d bits", validUntil.BitLen())
	}
	if validAfter.BitLen() > 48 {
		t.Fatalf("validAfter exceeds 48 bits: %d bits", validAfter.BitLen())
	}

	// Define the arguments for encoding validity
	uint48Ty, err := abi.NewType("uint48", "uint48", nil)
	if err != nil {
		t.Fatalf("Failed to create uint48 type: %v", err)
	}

	args := abi.Arguments{
		{Type: uint48Ty},
		{Type: uint48Ty},
	}

	// Encode the validity values
	validity, err := args.Pack(validUntil, validAfter)
	if err != nil {
		t.Fatalf("Failed to pack validity: %v", err)
	}

	// Create a mock signature (65 bytes: r(32) + s(32) + v(1))
	mockSig := make([]byte, 65)
	for i := range mockSig {
		mockSig[i] = byte(i) // Fill with test data
	}
	mockSig[64] = 27 // v value

	// Construct PaymasterAndData
	// Format: paymasterAddress (20 bytes) + validity (64 bytes: 32+32 ABI padded) + signature (65 bytes)
	data := append(paymasterAddr.Bytes(), validity...)
	data = append(data, mockSig...)

	// Print the encoding as hex string
	hexString := "0x" + hex.EncodeToString(data)
	fmt.Printf("\n=== PaymasterAndData Encoding ===\n")
	fmt.Printf("Paymaster Address: %s\n", paymasterAddr.Hex())
	fmt.Printf("Valid Until: %s (timestamp: %d)\n", validUntil.String(), validUntil.Int64())
	fmt.Printf("Valid After: %s (timestamp: %d)\n", validAfter.String(), validAfter.Int64())
	fmt.Printf("Validity length: %d bytes\n", len(validity))
	fmt.Printf("Validity (hex): 0x%s\n", hex.EncodeToString(validity))
	fmt.Printf("Total PaymasterAndData length: %d bytes\n", len(data))
	fmt.Printf("PaymasterAndData (hex): %s\n", hexString)
	fmt.Printf("================================\n\n")

	// Verify the structure
	// Note: ABI encoding pads each uint48 to 32 bytes, so validity is 64 bytes total
	expectedLen := 20 + 64 + 65 // address + validity (ABI padded) + signature
	if len(data) != expectedLen {
		t.Errorf("Expected PaymasterAndData length %d, got %d", expectedLen, len(data))
	}

	// Now parse the PaymasterAndData back
	t.Run("ParsePaymasterAndData", func(t *testing.T) {
		if len(data) < VALID_TIMESTAMP_OFFSET {
			t.Fatal("Data too short to contain address")
		}

		// Extract paymaster address (first 20 bytes)
		parsedAddr := common.BytesToAddress(data[0:VALID_TIMESTAMP_OFFSET])
		if parsedAddr != paymasterAddr {
			t.Errorf("Parsed address mismatch: expected %s, got %s", paymasterAddr.Hex(), parsedAddr.Hex())
		}

		// Extract validity (next 64 bytes - ABI encoded uint48 values are padded to 32 bytes each)
		// This matches the Solidity: paymasterAndData[VALID_TIMESTAMP_OFFSET:SIGNATURE_OFFSET]
		if len(data) < SIGNATURE_OFFSET {
			t.Fatal("Data too short to contain validity")
		}
		validityBytes := data[VALID_TIMESTAMP_OFFSET:SIGNATURE_OFFSET]

		// Decode the validity values
		uint48Ty, _ := abi.NewType("uint48", "uint48", nil)
		validityArgs := abi.Arguments{
			{Type: uint48Ty},
			{Type: uint48Ty},
		}

		unpacked, err := validityArgs.Unpack(validityBytes)
		if err != nil {
			t.Fatalf("Failed to unpack validity: %v", err)
		}

		if len(unpacked) != 2 {
			t.Fatalf("Expected 2 unpacked values, got %d", len(unpacked))
		}

		parsedValidUntil, ok := unpacked[0].(*big.Int)
		if !ok {
			t.Fatal("Failed to cast validUntil to *big.Int")
		}

		parsedValidAfter, ok := unpacked[1].(*big.Int)
		if !ok {
			t.Fatal("Failed to cast validAfter to *big.Int")
		}

		// Verify the parsed values match the original
		if parsedValidUntil.Cmp(validUntil) != 0 {
			t.Errorf("ValidUntil mismatch: expected %s, got %s", validUntil.String(), parsedValidUntil.String())
		}

		if parsedValidAfter.Cmp(validAfter) != 0 {
			t.Errorf("ValidAfter mismatch: expected %s, got %s", validAfter.String(), parsedValidAfter.String())
		}

		// Extract signature (remaining bytes after address + validity)
		// This matches the Solidity: paymasterAndData[SIGNATURE_OFFSET:]
		if len(data) < SIGNATURE_OFFSET+65 {
			t.Fatal("Data too short to contain signature")
		}
		parsedSig := data[SIGNATURE_OFFSET:]

		if len(parsedSig) != 65 {
			t.Errorf("Expected signature length 65, got %d", len(parsedSig))
		}

		// Verify signature bytes
		for i := range mockSig {
			if parsedSig[i] != mockSig[i] {
				t.Errorf("Signature byte mismatch at index %d: expected %d, got %d", i, mockSig[i], parsedSig[i])
			}
		}

		fmt.Printf("=== Parsing Verification ===\n")
		fmt.Printf("✓ Paymaster address parsed correctly: %s\n", parsedAddr.Hex())
		fmt.Printf("✓ ValidUntil parsed correctly: %s (timestamp: %d)\n", parsedValidUntil.String(), parsedValidUntil.Int64())
		fmt.Printf("✓ ValidAfter parsed correctly: %s (timestamp: %d)\n", parsedValidAfter.String(), parsedValidAfter.Int64())
		fmt.Printf("✓ Signature parsed correctly: %d bytes\n", len(parsedSig))
		fmt.Printf("============================\n\n")
	})
}

func TestPaymasterAndDataBoundaryValues(t *testing.T) {
	// Test with maximum uint48 values
	t.Run("MaxUint48Values", func(t *testing.T) {
		// Maximum value for uint48 is 2^48 - 1
		maxUint48 := new(big.Int).Sub(new(big.Int).Lsh(big.NewInt(1), 48), big.NewInt(1))

		paymasterAddr := common.HexToAddress("0xaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa")

		uint48Ty, err := abi.NewType("uint48", "uint48", nil)
		if err != nil {
			t.Fatalf("Failed to create uint48 type: %v", err)
		}

		args := abi.Arguments{
			{Type: uint48Ty},
			{Type: uint48Ty},
		}

		// Encode with max values
		validity, err := args.Pack(maxUint48, maxUint48)
		if err != nil {
			t.Fatalf("Failed to pack max validity: %v", err)
		}

		mockSig := make([]byte, 65)
		mockSig[64] = 28

		data := append(paymasterAddr.Bytes(), validity...)
		data = append(data, mockSig...)

		fmt.Printf("\n=== Boundary Test: Max uint48 ===\n")
		fmt.Printf("Max uint48 value: %s\n", maxUint48.String())
		fmt.Printf("Validity length: %d bytes\n", len(validity))
		fmt.Printf("Validity bytes (hex): 0x%s\n", hex.EncodeToString(validity))
		fmt.Printf("PaymasterAndData (hex): 0x%s\n", hex.EncodeToString(data))
		fmt.Printf("=================================\n\n")

		// Parse it back using Solidity contract offsets
		validityBytes := data[VALID_TIMESTAMP_OFFSET:SIGNATURE_OFFSET]
		unpacked, err := args.Unpack(validityBytes)
		if err != nil {
			t.Fatalf("Failed to unpack max validity: %v", err)
		}

		parsedVal1 := unpacked[0].(*big.Int)
		parsedVal2 := unpacked[1].(*big.Int)

		if parsedVal1.Cmp(maxUint48) != 0 {
			t.Errorf("First value mismatch: expected %s, got %s", maxUint48.String(), parsedVal1.String())
		}

		if parsedVal2.Cmp(maxUint48) != 0 {
			t.Errorf("Second value mismatch: expected %s, got %s", maxUint48.String(), parsedVal2.String())
		}

		fmt.Printf("✓ Max uint48 values encoded and decoded correctly\n\n")
	})

	t.Run("MinValues", func(t *testing.T) {
		// Test with minimum (zero) values
		paymasterAddr := common.HexToAddress("0xbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb")

		validUntil := big.NewInt(0)
		validAfter := big.NewInt(0)

		uint48Ty, _ := abi.NewType("uint48", "uint48", nil)
		args := abi.Arguments{
			{Type: uint48Ty},
			{Type: uint48Ty},
		}

		validity, err := args.Pack(validUntil, validAfter)
		if err != nil {
			t.Fatalf("Failed to pack zero validity: %v", err)
		}

		mockSig := make([]byte, 65)
		mockSig[64] = 27

		data := append(paymasterAddr.Bytes(), validity...)
		data = append(data, mockSig...)

		fmt.Printf("=== Boundary Test: Zero values ===\n")
		fmt.Printf("Validity length: %d bytes\n", len(validity))
		fmt.Printf("Validity bytes (hex): 0x%s\n", hex.EncodeToString(validity))
		fmt.Printf("PaymasterAndData (hex): 0x%s\n", hex.EncodeToString(data))
		fmt.Printf("==================================\n\n")

		// Parse it back using Solidity contract offsets
		validityBytes := data[VALID_TIMESTAMP_OFFSET:SIGNATURE_OFFSET]
		unpacked, err := args.Unpack(validityBytes)
		if err != nil {
			t.Fatalf("Failed to unpack zero validity: %v", err)
		}

		parsedVal1 := unpacked[0].(*big.Int)
		parsedVal2 := unpacked[1].(*big.Int)

		if parsedVal1.Cmp(big.NewInt(0)) != 0 {
			t.Errorf("First value should be 0, got %s", parsedVal1.String())
		}

		if parsedVal2.Cmp(big.NewInt(0)) != 0 {
			t.Errorf("Second value should be 0, got %s", parsedVal2.String())
		}

		fmt.Printf("✓ Zero values encoded and decoded correctly\n\n")
	})
}

func TestSolidityCompatibleParsing(t *testing.T) {
	// This test explicitly demonstrates parsing that matches the Solidity function:
	// function _parsePaymasterAndData(bytes calldata paymasterAndData)
	//     returns (uint48 validUntil, uint48 validAfter, bytes calldata signature)

	paymasterAddr := common.HexToAddress("0xCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCC")

	// Create validity values
	now := time.Now().Unix()
	validUntil := big.NewInt(now + 600) // 10 minutes from now
	validAfter := big.NewInt(now - 60)  // 1 minute ago

	// Encode validity using ABI
	uint48Ty, _ := abi.NewType("uint48", "uint48", nil)
	args := abi.Arguments{
		{Type: uint48Ty},
		{Type: uint48Ty},
	}
	validity, err := args.Pack(validUntil, validAfter)
	if err != nil {
		t.Fatalf("Failed to pack validity: %v", err)
	}

	// Create mock signature
	mockSig := make([]byte, 65)
	for i := 0; i < 32; i++ {
		mockSig[i] = 0xff // r component
	}
	for i := 32; i < 64; i++ {
		mockSig[i] = 0xaa // s component
	}
	mockSig[64] = 27 // v component

	// Construct PaymasterAndData
	paymasterAndData := append(paymasterAddr.Bytes(), validity...)
	paymasterAndData = append(paymasterAndData, mockSig...)

	fmt.Printf("\n=== Solidity-Compatible Parsing Test ===\n")
	fmt.Printf("PaymasterAndData: 0x%s\n", hex.EncodeToString(paymasterAndData))
	fmt.Printf("Total length: %d bytes\n", len(paymasterAndData))
	fmt.Printf("\nParsing using Solidity offsets:\n")
	fmt.Printf("  VALID_TIMESTAMP_OFFSET = %d\n", VALID_TIMESTAMP_OFFSET)
	fmt.Printf("  SIGNATURE_OFFSET = %d\n", SIGNATURE_OFFSET)
	fmt.Printf("========================================\n\n")

	// Parse using the same logic as Solidity contract
	// (validUntil, validAfter) = abi.decode(
	//     paymasterAndData[VALID_TIMESTAMP_OFFSET:SIGNATURE_OFFSET],
	//     (uint48, uint48)
	// );
	validityBytes := paymasterAndData[VALID_TIMESTAMP_OFFSET:SIGNATURE_OFFSET]
	unpacked, err := args.Unpack(validityBytes)
	if err != nil {
		t.Fatalf("Failed to decode validity: %v", err)
	}

	parsedValidUntil := unpacked[0].(*big.Int)
	parsedValidAfter := unpacked[1].(*big.Int)

	// signature = paymasterAndData[SIGNATURE_OFFSET:];
	signature := paymasterAndData[SIGNATURE_OFFSET:]

	// Verify parsed values
	if parsedValidUntil.Cmp(validUntil) != 0 {
		t.Errorf("ValidUntil mismatch: expected %s, got %s", validUntil.String(), parsedValidUntil.String())
	}

	if parsedValidAfter.Cmp(validAfter) != 0 {
		t.Errorf("ValidAfter mismatch: expected %s, got %s", validAfter.String(), parsedValidAfter.String())
	}

	if len(signature) != 65 {
		t.Errorf("Signature length mismatch: expected 65, got %d", len(signature))
	}

	for i := range mockSig {
		if signature[i] != mockSig[i] {
			t.Errorf("Signature byte mismatch at index %d", i)
			break
		}
	}

	fmt.Printf("✓ Successfully parsed using Solidity-compatible offsets:\n")
	fmt.Printf("  validUntil: %s (timestamp: %d)\n", parsedValidUntil.String(), parsedValidUntil.Int64())
	fmt.Printf("  validAfter: %s (timestamp: %d)\n", parsedValidAfter.String(), parsedValidAfter.Int64())
	fmt.Printf("  signature: %d bytes\n", len(signature))
	fmt.Printf("  signature (hex): 0x%s\n\n", hex.EncodeToString(signature))
}

func TestParseRealPaymasterAndData(t *testing.T) {
	// Real PaymasterAndData value from production for debugging
	hexData := "0xe69c843898e21c0e95ea7dd310cd850aac0ab897000000000000000000000000000000000000000000000000000000006901f5550000000000000000000000000000000000000000000000000000000068f8bacbd9950053efac34e58f9923ba98527060029c9153fa5960809809572b486cc32d63a60e19f4d5612130912c07c95587e5352f1ee987a3bddd17285eb0cf9e8d401c"

	// Remove 0x prefix if present
	if len(hexData) > 2 && hexData[:2] == "0x" {
		hexData = hexData[2:]
	}

	// Decode hex string to bytes
	paymasterAndData, err := hex.DecodeString(hexData)
	if err != nil {
		t.Fatalf("Failed to decode hex string: %v", err)
	}

	fmt.Printf("\n=== Parsing Real PaymasterAndData ===\n")
	fmt.Printf("Raw data (hex): 0x%s\n", hex.EncodeToString(paymasterAndData))
	fmt.Printf("Total length: %d bytes\n\n", len(paymasterAndData))

	// Verify minimum length
	if len(paymasterAndData) < SIGNATURE_OFFSET {
		t.Fatalf("PaymasterAndData too short: expected at least %d bytes, got %d", SIGNATURE_OFFSET, len(paymasterAndData))
	}

	// 1. Extract Paymaster Address (bytes 0-20)
	paymasterAddr := common.BytesToAddress(paymasterAndData[0:VALID_TIMESTAMP_OFFSET])
	fmt.Printf("📍 Paymaster Address:\n")
	fmt.Printf("   Address: %s\n", paymasterAddr.Hex())
	fmt.Printf("   Bytes (hex): 0x%s\n\n", hex.EncodeToString(paymasterAndData[0:VALID_TIMESTAMP_OFFSET]))

	// 2. Extract and decode validity period (bytes 20-84)
	validityBytes := paymasterAndData[VALID_TIMESTAMP_OFFSET:SIGNATURE_OFFSET]
	fmt.Printf("⏰ Validity Period:\n")
	fmt.Printf("   Raw bytes (hex): 0x%s\n", hex.EncodeToString(validityBytes))
	fmt.Printf("   Length: %d bytes\n", len(validityBytes))

	// Decode using ABI
	uint48Ty, _ := abi.NewType("uint48", "uint48", nil)
	args := abi.Arguments{
		{Type: uint48Ty},
		{Type: uint48Ty},
	}

	unpacked, err := args.Unpack(validityBytes)
	if err != nil {
		t.Fatalf("Failed to unpack validity: %v", err)
	}

	validUntil := unpacked[0].(*big.Int)
	validAfter := unpacked[1].(*big.Int)

	// Convert timestamps to time.Time for human-readable output
	validUntilTime := time.Unix(validUntil.Int64(), 0)
	validAfterTime := time.Unix(validAfter.Int64(), 0)
	now := time.Now()

	fmt.Printf("   Valid Until: %d (%s)\n", validUntil.Int64(), validUntil.String())
	fmt.Printf("               Time: %s\n", validUntilTime.Format(time.RFC3339))
	if validUntilTime.After(now) {
		fmt.Printf("               Status: ✓ Valid (expires in %s)\n", validUntilTime.Sub(now).Round(time.Second))
	} else {
		fmt.Printf("               Status: ✗ Expired (%s ago)\n", now.Sub(validUntilTime).Round(time.Second))
	}

	fmt.Printf("   Valid After: %d (%s)\n", validAfter.Int64(), validAfter.String())
	fmt.Printf("               Time: %s\n", validAfterTime.Format(time.RFC3339))
	if validAfterTime.Before(now) {
		fmt.Printf("               Status: ✓ Valid (started %s ago)\n", now.Sub(validAfterTime).Round(time.Second))
	} else {
		fmt.Printf("               Status: ✗ Not yet valid (starts in %s)\n", validAfterTime.Sub(now).Round(time.Second))
	}
	fmt.Println()

	// 3. Extract signature (bytes 84 onwards)
	signature := paymasterAndData[SIGNATURE_OFFSET:]
	fmt.Printf("✍️  Signature:\n")
	fmt.Printf("   Length: %d bytes\n", len(signature))
	fmt.Printf("   Full signature (hex): 0x%s\n", hex.EncodeToString(signature))

	if len(signature) == 65 {
		r := signature[0:32]
		s := signature[32:64]
		v := signature[64]

		fmt.Printf("   r (32 bytes): 0x%s\n", hex.EncodeToString(r))
		fmt.Printf("   s (32 bytes): 0x%s\n", hex.EncodeToString(s))
		fmt.Printf("   v (1 byte):   0x%02x (%d)\n", v, v)

		if v == 27 || v == 28 {
			fmt.Printf("   Status: ✓ Valid v value\n")
		} else {
			fmt.Printf("   Status: ⚠️  Unusual v value (expected 27 or 28)\n")
		}
	} else {
		fmt.Printf("   Status: ⚠️  Unexpected signature length (expected 65 bytes)\n")
	}

	fmt.Printf("\n=====================================\n\n")

	// Check specific submission time
	submittedAt := int64(1761131279)
	submittedTime := time.Unix(submittedAt, 0)

	fmt.Printf("🕐 Validation Check for Submission Time:\n")
	fmt.Printf("   Submitted at: %d\n", submittedAt)
	fmt.Printf("   Submitted time: %s\n", submittedTime.Format(time.RFC3339))
	fmt.Printf("\n")

	// Check if submission was within valid time window
	isAfterValidAfter := submittedAt > validAfter.Int64()
	isBeforeValidUntil := submittedAt < validUntil.Int64()
	isValid := isAfterValidAfter && isBeforeValidUntil

	fmt.Printf("   Checking validity window:\n")
	fmt.Printf("   ├─ ValidAfter:  %d (%s)\n", validAfter.Int64(), validAfterTime.Format(time.RFC3339))
	fmt.Printf("   ├─ Submitted:   %d (%s)\n", submittedAt, submittedTime.Format(time.RFC3339))
	fmt.Printf("   └─ ValidUntil:  %d (%s)\n", validUntil.Int64(), validUntilTime.Format(time.RFC3339))
	fmt.Printf("\n")

	if isAfterValidAfter {
		diff := submittedAt - validAfter.Int64()
		fmt.Printf("   ✓ Submitted %d seconds AFTER validAfter (OK)\n", diff)
	} else {
		diff := validAfter.Int64() - submittedAt
		fmt.Printf("   ✗ Submitted %d seconds BEFORE validAfter (TOO EARLY)\n", diff)
	}

	if isBeforeValidUntil {
		diff := validUntil.Int64() - submittedAt
		fmt.Printf("   ✓ Submitted %d seconds BEFORE validUntil (OK)\n", diff)
	} else {
		diff := submittedAt - validUntil.Int64()
		fmt.Printf("   ✗ Submitted %d seconds AFTER validUntil (EXPIRED)\n", diff)
	}

	fmt.Printf("\n")
	if isValid {
		fmt.Printf("   ✅ RESULT: PaymasterAndData was VALID at submission time\n")
	} else {
		fmt.Printf("   ❌ RESULT: PaymasterAndData was INVALID at submission time\n")
	}
	fmt.Printf("\n=====================================\n\n")

	// Verify structure
	expectedLength := VALID_TIMESTAMP_OFFSET + (SIGNATURE_OFFSET - VALID_TIMESTAMP_OFFSET) + len(signature)
	if len(paymasterAndData) != expectedLength {
		t.Logf("Warning: Length mismatch. Expected %d, got %d", expectedLength, len(paymasterAndData))
	}

	// Assert it was valid
	if !isValid {
		t.Errorf("PaymasterAndData was invalid at submission time %d", submittedAt)
	}
}

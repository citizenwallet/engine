package engine

import (
	"reflect"
	"testing"
)

func TestEvent_ParseEventSignature(t *testing.T) {
	tests := []struct {
		name          string
		signature     string
		wantEventName string
		wantArgNames  []string
		wantArgTypes  []string
	}{
		{
			name:          "Full signature with named arguments and spaces",
			signature:     "Transfer(from address, to address, value uint256)",
			wantEventName: "Transfer",
			wantArgNames:  []string{"from", "to", "value"},
			wantArgTypes:  []string{"address", "address", "uint256"},
		},
		{
			name:          "Full signature with named arguments",
			signature:     "Transfer(from address,to address,value uint256)",
			wantEventName: "Transfer",
			wantArgNames:  []string{"from", "to", "value"},
			wantArgTypes:  []string{"address", "address", "uint256"},
		},
		{
			name:          "Compact signature without named arguments",
			signature:     "Transfer(address,address,uint256)",
			wantEventName: "Transfer",
			wantArgNames:  []string{"0", "1", "2"},
			wantArgTypes:  []string{"address", "address", "uint256"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := &Event{EventSignature: tt.signature}
			gotEventName, gotArgNames, gotArgTypes := e.ParseEventSignature()

			if gotEventName != tt.wantEventName {
				t.Errorf("Event.ParseEventSignature() eventName = %v, want %v", gotEventName, tt.wantEventName)
			}

			if !reflect.DeepEqual(gotArgNames, tt.wantArgNames) {
				t.Errorf("Event.ParseEventSignature() argNames = %v, want %v", gotArgNames, tt.wantArgNames)
			}

			if !reflect.DeepEqual(gotArgTypes, tt.wantArgTypes) {
				t.Errorf("Event.ParseEventSignature() argTypes = %v, want %v", gotArgTypes, tt.wantArgTypes)
			}
		})
	}
}

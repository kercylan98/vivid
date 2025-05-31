package processor_test

import (
    "github.com/kercylan98/vivid/engine/v1/internal/processor"
    "testing"
)

func TestNewUnitIdentifier(t *testing.T) {
    processor.NewUnitIdentifier("127.0.0.1:8080", "/vivid")
    processor.NewUnitIdentifier("", "/vivid")
    processor.NewUnitIdentifier("", "")
    processor.NewUnitIdentifier("", "/")
    processor.NewUnitIdentifier("", "/vivid/123")
}

func TestUnitIdentifier_GetAddress(t *testing.T) {
    unitIdentifier := processor.NewUnitIdentifier("127.0.0.1:8080", "/vivid")
    if unitIdentifier.GetAddress() != "127.0.0.1:8080" {
        t.Error("GetAddress() error")
    }
}

func TestUnitIdentifier_GetPath(t *testing.T) {
    var tests = []struct {
        unitIdentifier processor.UnitIdentifier
        path           string
    }{
        {processor.NewUnitIdentifier("127.0.0.1:8080", "/vivid"), "/vivid"},
        {processor.NewUnitIdentifier("127.0.0.1:8080", ""), "/"},
        {processor.NewUnitIdentifier("127.0.0.1:8080", "/"), "/"},
        {processor.NewUnitIdentifier("127.0.0.1:8080", "/vivid/123"), "/vivid/123"},
        {processor.NewUnitIdentifier("127.0.0.1:8080", "/vivid/"), "/vivid"},
        {processor.NewUnitIdentifier("127.0.0.1:8080", "//vivid/"), "/vivid"},
        {processor.NewUnitIdentifier("127.0.0.1:8080", "vivid"), "/vivid"},
    }

    for _, test := range tests {
        if test.unitIdentifier.GetPath() != test.path {
            t.Error("GetPath() error")
        }
    }
}

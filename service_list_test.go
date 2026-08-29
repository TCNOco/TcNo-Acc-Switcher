package main

import (
	"strings"
	"testing"

	"github.com/wailsapp/wails/v3/pkg/application"
)

type serviceInvariantFixture struct{}

func TestServiceListContainsOnlyConcreteInstances(t *testing.T) {
	if err := validateWailsServices(serviceList()); err != nil {
		t.Fatal(err)
	}
}

func TestValidateWailsServicesRejectsNilInstance(t *testing.T) {
	err := validateWailsServices([]application.Service{{}})
	if err == nil {
		t.Fatal("zero-value service was accepted")
	}
	if !strings.Contains(err.Error(), "index 0") {
		t.Fatalf("error %q does not identify the invalid service", err)
	}
}

func TestValidateWailsServicesRejectsTypedNilInstance(t *testing.T) {
	var instance *serviceInvariantFixture
	err := validateWailsServices([]application.Service{
		application.NewService(&serviceInvariantFixture{}),
		application.NewService(instance),
	})
	if err == nil {
		t.Fatal("typed-nil service was accepted")
	}
	if !strings.Contains(err.Error(), "index 1") || !strings.Contains(err.Error(), "*main.serviceInvariantFixture") {
		t.Fatalf("error %q does not identify the typed-nil service", err)
	}
}

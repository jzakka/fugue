package bot

import "testing"

func TestHarvesterConsumer_HasMediaValidator_DefaultFalse(t *testing.T) {
	c := NewHarvesterConsumer(nil, nil, nil, nil, nil, NewMockPipeline())
	if c.HasMediaValidator() {
		t.Fatal("HasMediaValidator must be false before WithMediaValidator is called")
	}
}

func TestHarvesterConsumer_HasMediaValidator_AfterWith(t *testing.T) {
	c := NewHarvesterConsumer(nil, nil, nil, nil, nil, NewMockPipeline()).
		WithMediaValidator(NewDefaultMediaValidator())
	if !c.HasMediaValidator() {
		t.Fatal("HasMediaValidator must be true after WithMediaValidator")
	}
}

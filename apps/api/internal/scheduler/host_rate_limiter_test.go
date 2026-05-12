package scheduler

import (
	"bytes"
	"log"
	"strings"
	"sync"
	"testing"
	"time"
)

// 4.1
func TestAllow_NewHostUsesDefaults(t *testing.T) {
	l := NewHostRateLimiter(FactoryDefaultRatePerSec, FactoryDefaultBurst, true)
	if !l.Allow("h1") {
		t.Fatalf("first Allow on new host should be true with default burst=5")
	}
}

// 4.2
func TestAllow_BurstThenBlocks(t *testing.T) {
	l := NewHostRateLimiter(FactoryDefaultRatePerSec, FactoryDefaultBurst, true)
	for i := 0; i < FactoryDefaultBurst; i++ {
		if !l.Allow("h1") {
			t.Fatalf("call %d should pass within burst", i)
		}
	}
	if l.Allow("h1") {
		t.Fatalf("call after burst exhausted should be false")
	}
}

// 4.3
func TestAllow_RefillsOverTime(t *testing.T) {
	// rate 50/s ⇒ 1 token every 20ms; burst 1 to make refill observable quickly.
	l := NewHostRateLimiter(50.0, 1, true)
	if !l.Allow("h1") {
		t.Fatalf("first call should pass")
	}
	if l.Allow("h1") {
		t.Fatalf("second call immediately after burst should be blocked")
	}
	time.Sleep(40 * time.Millisecond)
	if !l.Allow("h1") {
		t.Fatalf("after refill window, call should pass again")
	}
}

// 4.4
func TestSetHostRate_ImmediatelyEffective(t *testing.T) {
	l := NewHostRateLimiter(FactoryDefaultRatePerSec, FactoryDefaultBurst, true)
	// Existing host: prime then re-set.
	l.Allow("existing")
	l.SetHostRate("existing", 100.0, 2)
	for i := 0; i < 2; i++ {
		if !l.Allow("existing") {
			t.Fatalf("after SetHostRate burst=2, Allow %d should pass", i)
		}
	}
	if l.Allow("existing") {
		t.Fatalf("third Allow should be blocked under new burst=2")
	}
	// New host.
	l.SetHostRate("brand-new", 100.0, 3)
	for i := 0; i < 3; i++ {
		if !l.Allow("brand-new") {
			t.Fatalf("new host: Allow %d should pass under burst=3", i)
		}
	}
	if l.Allow("brand-new") {
		t.Fatalf("new host: Allow beyond burst should fail")
	}
}

// 4.5
func TestSetHostRate_InvalidInputsSubstituteDefaults(t *testing.T) {
	var buf bytes.Buffer
	old := log.Writer()
	log.SetOutput(&buf)
	defer log.SetOutput(old)

	l := NewHostRateLimiter(FactoryDefaultRatePerSec, FactoryDefaultBurst, true)
	cases := []struct {
		name string
		r    float64
		b    int
	}{
		{"rate=0", 0, 5},
		{"burst=0", 1, 0},
		{"both negative", -1, -1},
	}
	for _, tc := range cases {
		buf.Reset()
		host := "bad-" + tc.name
		l.SetHostRate(host, tc.r, tc.b)
		// After substitution, host should behave like factory defaults (burst=5).
		passed := 0
		for i := 0; i < FactoryDefaultBurst; i++ {
			if l.Allow(host) {
				passed++
			}
		}
		if passed != FactoryDefaultBurst {
			t.Fatalf("%s: expected %d initial Allow=true, got %d", tc.name, FactoryDefaultBurst, passed)
		}
		if !strings.Contains(buf.String(), "WARN") {
			t.Fatalf("%s: expected WARN log, got %q", tc.name, buf.String())
		}
	}
}

// 4.5b: operator-configured defaults are used for substitution.
func TestSetHostRate_InvalidUsesOperatorDefaults(t *testing.T) {
	var buf bytes.Buffer
	old := log.Writer()
	log.SetOutput(&buf)
	defer log.SetOutput(old)

	l := NewHostRateLimiter(0.5, 3, true)
	l.SetHostRate("h", 0, 5)
	passed := 0
	for i := 0; i < 4; i++ {
		if l.Allow("h") {
			passed++
		}
	}
	if passed != 3 {
		t.Fatalf("expected operator burst=3 to apply after substitution, got %d", passed)
	}
}

// 4.6
func TestAllow_DisabledAlwaysTrue(t *testing.T) {
	l := NewHostRateLimiter(FactoryDefaultRatePerSec, FactoryDefaultBurst, false)
	for i := 0; i < 100; i++ {
		if !l.Allow("anyhost") {
			t.Fatalf("disabled limiter should always return true (i=%d)", i)
		}
	}
}

// Spec scenario: 호스트 키는 frontier host 컬럼 값 그대로 사용된다.
// scheduler가 대소문자/`www.` prefix를 변형하지 않음을 보장하는 회귀 방지 테스트.
func TestAllow_HostKeyNotNormalized(t *testing.T) {
	l := NewHostRateLimiter(FactoryDefaultRatePerSec, FactoryDefaultBurst, true)
	mixed := "www.Example.COM"
	lower := "www.example.com"
	// Drain mixed-case host's burst.
	for i := 0; i < FactoryDefaultBurst; i++ {
		if !l.Allow(mixed) {
			t.Fatalf("mixed-case host: Allow %d should pass within burst", i)
		}
	}
	if l.Allow(mixed) {
		t.Fatalf("mixed-case host: Allow after burst should be blocked")
	}
	// Lowercase variant must be a separate bucket (no normalization).
	if !l.Allow(lower) {
		t.Fatalf("lowercase host should have an independent fresh bucket")
	}
}

// Spec scenario: 비활성화 상태에서 토큰 소비가 발생하지 않는다.
// 활성→비활성 100회→재활성 시 burst 가득 상태가 유지되어야 한다.
func TestAllow_DisabledDoesNotConsumeTokens(t *testing.T) {
	l := NewHostRateLimiter(FactoryDefaultRatePerSec, FactoryDefaultBurst, true)
	// Switch to disabled and call 100 times.
	l.enabled = false
	for i := 0; i < 100; i++ {
		if !l.Allow("h") {
			t.Fatalf("disabled: Allow %d must be true", i)
		}
	}
	// Re-enable; bucket for "h" was never created during disabled state,
	// so first burst worth of Allow must all succeed.
	l.enabled = true
	for i := 0; i < FactoryDefaultBurst; i++ {
		if !l.Allow("h") {
			t.Fatalf("after re-enable: Allow %d must pass within burst (no consumption during disabled)", i)
		}
	}
	if l.Allow("h") {
		t.Fatalf("after re-enable: burst should be exhausted at exactly FactoryDefaultBurst calls")
	}
}

// 4.7
func TestConcurrent_AllowAndSetHostRate(t *testing.T) {
	l := NewHostRateLimiter(FactoryDefaultRatePerSec, FactoryDefaultBurst, true)
	var wg sync.WaitGroup
	hosts := []string{"a", "b", "c", "d"}
	for w := 0; w < 32; w++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			h := hosts[id%len(hosts)]
			for i := 0; i < 200; i++ {
				if i%50 == 0 {
					l.SetHostRate(h, float64(1+id%5), 1+id%5)
				} else {
					_ = l.Allow(h)
				}
			}
		}(w)
	}
	wg.Wait()
}

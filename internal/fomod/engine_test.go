package fomod

import "testing"

const sampleModuleConfig = `<?xml version="1.0" encoding="utf-8"?>
<config>
  <moduleName>Sample Mod</moduleName>
  <requiredInstallFiles>
    <file source="core/core.lua" destination="LuaMods/Sample/core.lua"/>
  </requiredInstallFiles>
  <installSteps order="Explicit">
    <installStep name="Quality">
      <optionalFileGroups order="Explicit">
        <group name="Texture Quality" type="SelectExactlyOne">
          <plugins order="Explicit">
            <plugin name="High">
              <description>High quality textures</description>
              <files>
                <file source="textures/high.dds" destination="Textures/quality.dds"/>
              </files>
              <conditionFlags>
                <flag name="quality">high</flag>
              </conditionFlags>
              <typeDescriptor><type name="Optional"/></typeDescriptor>
            </plugin>
            <plugin name="Low">
              <description>Low quality textures</description>
              <files>
                <file source="textures/low.dds" destination="Textures/quality.dds"/>
              </files>
              <conditionFlags>
                <flag name="quality">low</flag>
              </conditionFlags>
              <typeDescriptor><type name="Optional"/></typeDescriptor>
            </plugin>
          </plugins>
        </group>
      </optionalFileGroups>
    </installStep>
    <installStep name="Extras">
      <visible>
        <flagDependency flag="quality" value="high"/>
      </visible>
      <optionalFileGroups order="Explicit">
        <group name="Bonus" type="SelectAny">
          <plugins order="Explicit">
            <plugin name="Extra Sounds">
              <description>Bonus sound pack</description>
              <files>
                <file source="sounds/extra.bank" destination="Sounds/extra.bank"/>
              </files>
              <typeDescriptor><type name="Optional"/></typeDescriptor>
            </plugin>
          </plugins>
        </group>
      </optionalFileGroups>
    </installStep>
  </installSteps>
  <conditionalFileInstalls>
    <patterns>
      <pattern>
        <dependencies operator="And">
          <flagDependency flag="quality" value="high"/>
        </dependencies>
        <files>
          <file source="textures/high_extra.dds" destination="Textures/extra.dds"/>
        </files>
      </pattern>
    </patterns>
  </conditionalFileInstalls>
</config>`

func mustParse(t *testing.T) *Config {
	cfg, err := Parse([]byte(sampleModuleConfig))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	return cfg
}

func TestParseBasicStructure(t *testing.T) {
	cfg := mustParse(t)
	if cfg.ModuleName != "Sample Mod" {
		t.Fatalf("ModuleName = %q", cfg.ModuleName)
	}
	if len(cfg.InstallSteps.Steps) != 2 {
		t.Fatalf("expected 2 install steps, got %d", len(cfg.InstallSteps.Steps))
	}
	if len(cfg.RequiredInstallFiles.Files) != 1 {
		t.Fatalf("expected 1 required file, got %d", len(cfg.RequiredInstallFiles.Files))
	}
}

func TestGroupValidation(t *testing.T) {
	cfg := mustParse(t)
	s := NewSession(cfg)

	if err := s.SelectStep(map[string][]string{"Texture Quality": {"High", "Low"}}); err == nil {
		t.Fatal("expected error selecting two plugins from a SelectExactlyOne group")
	}
	if err := s.SelectStep(map[string][]string{"Texture Quality": {}}); err == nil {
		t.Fatal("expected error selecting zero plugins from a SelectExactlyOne group")
	}
}

func TestVisibilityAndFinalizeHighPath(t *testing.T) {
	cfg := mustParse(t)
	s := NewSession(cfg)

	step, ok := s.CurrentStep()
	if !ok || step.Name != "Quality" {
		t.Fatalf("expected first step Quality, got %+v ok=%v", step, ok)
	}
	if err := s.SelectStep(map[string][]string{"Texture Quality": {"High"}}); err != nil {
		t.Fatalf("SelectStep: %v", err)
	}

	// Choosing "High" sets flag quality=high, which makes the Extras step visible.
	step, ok = s.CurrentStep()
	if !ok || step.Name != "Extras" {
		t.Fatalf("expected Extras step to become visible, got %+v ok=%v", step, ok)
	}
	if err := s.SelectStep(map[string][]string{"Bonus": {"Extra Sounds"}}); err != nil {
		t.Fatalf("SelectStep: %v", err)
	}

	if !s.Done() {
		t.Fatal("expected wizard to be done")
	}

	files, err := s.Finalize()
	if err != nil {
		t.Fatalf("Finalize: %v", err)
	}

	want := map[string]string{
		"LuaMods/Sample/core.lua": "core/core.lua",
		"Textures/quality.dds":    "textures/high.dds",
		"Sounds/extra.bank":       "sounds/extra.bank",
		"Textures/extra.dds":      "textures/high_extra.dds",
	}
	if len(files) != len(want) {
		t.Fatalf("expected %d resolved files, got %d: %+v", len(want), len(files), files)
	}
	for _, f := range files {
		src, ok := want[f.Dest]
		if !ok {
			t.Fatalf("unexpected destination %q in result", f.Dest)
		}
		if src != f.Source {
			t.Fatalf("dest %q: expected source %q, got %q", f.Dest, src, f.Source)
		}
	}
}

func TestVisibilityLowPathSkipsExtrasAndConditional(t *testing.T) {
	cfg := mustParse(t)
	s := NewSession(cfg)

	if err := s.SelectStep(map[string][]string{"Texture Quality": {"Low"}}); err != nil {
		t.Fatalf("SelectStep: %v", err)
	}

	// Extras step requires quality=high, so it should be skipped — wizard is done.
	if !s.Done() {
		step, _ := s.CurrentStep()
		t.Fatalf("expected wizard done after low-quality pick, got pending step %+v", step)
	}

	files, err := s.Finalize()
	if err != nil {
		t.Fatalf("Finalize: %v", err)
	}
	for _, f := range files {
		if f.Dest == "Sounds/extra.bank" || f.Dest == "Textures/extra.dds" {
			t.Fatalf("did not expect %q to be installed on low-quality path", f.Dest)
		}
	}
	if len(files) != 2 {
		t.Fatalf("expected 2 files (required + texture choice), got %d: %+v", len(files), files)
	}
}

func TestFinalizeBeforeDoneFails(t *testing.T) {
	cfg := mustParse(t)
	s := NewSession(cfg)
	if _, err := s.Finalize(); err == nil {
		t.Fatal("expected Finalize to fail before the wizard is complete")
	}
}

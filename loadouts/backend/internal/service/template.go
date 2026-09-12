package service

import (
	"context"
	"fmt"
	"strings"

	"github.com/gmccloskey/loadouts/backend/internal/core"
	"github.com/gmccloskey/loadouts/backend/internal/db"
)

// TemplateService owns Templates and their immutable versions.
//
// Publishing a change never mutates an existing version: it appends a new one and moves the
// template's LatestVersion pointer. Loadouts pin (template_id, template_version), so an
// author can evolve a template freely without breaking anyone else's saved loadout.
type TemplateService struct {
	store     db.Store
	community *CommunityService
}

func NewTemplateService(store db.Store, community *CommunityService) *TemplateService {
	return &TemplateService{store: store, community: community}
}

// EnsureFreeform creates the built-in platform template if it does not exist. It is the
// "base generic template" that lets a user add any items without structure.
func (s *TemplateService) EnsureFreeform(ctx context.Context) error {
	if _, err := s.store.GetTemplate(ctx, core.FreeformTemplateID); err == nil {
		return nil
	}

	tmpl := core.Template{
		ID:            core.FreeformTemplateID,
		Name:          "Freeform",
		Description:   "No structure. Add any items you like and organize them yourself.",
		OwnerType:     core.OwnerPlatform,
		LatestVersion: 1,
		IsPublic:      true,
	}
	if err := s.store.CreateTemplate(ctx, tmpl); err != nil {
		return err
	}
	return s.store.CreateTemplateVersion(ctx, core.TemplateVersion{
		TemplateID: tmpl.ID,
		Version:    1,
		Slots:      core.SlotList{},
		Changelog:  "Initial version.",
	})
}

// Create registers a template and publishes version 1 in one shot.
func (s *TemplateService) Create(ctx context.Context, actorProfileID string, tmpl core.Template, slots core.SlotList, changelog string) (core.TemplateDetail, error) {
	if actorProfileID == "" {
		return core.TemplateDetail{}, fmt.Errorf("%w: a profile is required to create a template", core.ErrForbidden)
	}
	if strings.TrimSpace(tmpl.Name) == "" {
		return core.TemplateDetail{}, fmt.Errorf("%w: name is required", core.ErrInvalid)
	}

	switch tmpl.OwnerType {
	case core.OwnerCommunity:
		// Community-owned templates require admin rights in that community.
		if tmpl.CommunityID == "" {
			tmpl.CommunityID = tmpl.OwnerID
		}
		if err := s.community.RequireAdmin(ctx, tmpl.CommunityID, actorProfileID); err != nil {
			return core.TemplateDetail{}, err
		}
		tmpl.OwnerID = tmpl.CommunityID
	case core.OwnerPlatform:
		return core.TemplateDetail{}, fmt.Errorf("%w: platform templates are seeded, not user-created", core.ErrForbidden)
	default:
		tmpl.OwnerType = core.OwnerProfile
		tmpl.OwnerID = actorProfileID
	}

	if err := normalizeSlots(slots); err != nil {
		return core.TemplateDetail{}, err
	}

	tmpl.ID = core.NewID("tpl")
	tmpl.LatestVersion = 1
	if err := s.store.CreateTemplate(ctx, tmpl); err != nil {
		return core.TemplateDetail{}, err
	}

	version := core.TemplateVersion{
		TemplateID: tmpl.ID,
		Version:    1,
		Slots:      slots,
		Changelog:  defaultString(changelog, "Initial version."),
	}
	if err := s.store.CreateTemplateVersion(ctx, version); err != nil {
		return core.TemplateDetail{}, err
	}

	return core.TemplateDetail{Template: tmpl, Version: version, Versions: []int{1}}, nil
}

// PublishVersion appends an immutable new version and advances LatestVersion.
func (s *TemplateService) PublishVersion(ctx context.Context, actorProfileID, templateID string, slots core.SlotList, changelog string) (core.TemplateDetail, error) {
	tmpl, err := s.store.GetTemplate(ctx, templateID)
	if err != nil {
		return core.TemplateDetail{}, fmt.Errorf("%w: template %s", core.ErrNotFound, templateID)
	}
	if err := s.requireEditor(ctx, tmpl, actorProfileID); err != nil {
		return core.TemplateDetail{}, err
	}
	if err := normalizeSlots(slots); err != nil {
		return core.TemplateDetail{}, err
	}

	next := core.TemplateVersion{
		TemplateID: tmpl.ID,
		Version:    tmpl.LatestVersion + 1,
		Slots:      slots,
		Changelog:  defaultString(changelog, fmt.Sprintf("Version %d", tmpl.LatestVersion+1)),
	}
	if err := s.store.CreateTemplateVersion(ctx, next); err != nil {
		return core.TemplateDetail{}, err
	}

	tmpl.LatestVersion = next.Version
	if err := s.store.UpdateTemplate(ctx, tmpl); err != nil {
		return core.TemplateDetail{}, err
	}

	return s.Detail(ctx, tmpl.ID, next.Version)
}

// requireEditor allows the owning profile, or an admin of the owning community.
func (s *TemplateService) requireEditor(ctx context.Context, tmpl core.Template, actorProfileID string) error {
	switch tmpl.OwnerType {
	case core.OwnerProfile:
		if tmpl.OwnerID != actorProfileID {
			return fmt.Errorf("%w: only @owner may publish new versions of this template", core.ErrForbidden)
		}
		return nil
	case core.OwnerCommunity:
		return s.community.RequireAdmin(ctx, tmpl.CommunityID, actorProfileID)
	default:
		return fmt.Errorf("%w: platform templates are immutable", core.ErrForbidden)
	}
}

// Detail returns a template with a specific version. Pass version <= 0 for the latest.
func (s *TemplateService) Detail(ctx context.Context, templateID string, version int) (core.TemplateDetail, error) {
	tmpl, err := s.store.GetTemplate(ctx, templateID)
	if err != nil {
		return core.TemplateDetail{}, fmt.Errorf("%w: template %s", core.ErrNotFound, templateID)
	}
	if version <= 0 {
		version = tmpl.LatestVersion
	}

	tv, err := s.store.GetTemplateVersion(ctx, templateID, version)
	if err != nil {
		return core.TemplateDetail{}, fmt.Errorf("%w: template %s v%d", core.ErrNotFound, templateID, version)
	}

	all, err := s.store.ListTemplateVersions(ctx, templateID)
	if err != nil {
		return core.TemplateDetail{}, err
	}
	numbers := make([]int, 0, len(all))
	for _, v := range all {
		numbers = append(numbers, v.Version)
	}

	return core.TemplateDetail{Template: tmpl, Version: tv, Versions: numbers}, nil
}

func (s *TemplateService) List(ctx context.Context, q core.TemplateQuery) ([]core.TemplateDetail, error) {
	templates, err := s.store.ListTemplates(ctx, q)
	if err != nil {
		return nil, err
	}
	details := make([]core.TemplateDetail, 0, len(templates))
	for _, t := range templates {
		detail, err := s.Detail(ctx, t.ID, t.LatestVersion)
		if err != nil {
			continue
		}
		details = append(details, detail)
	}
	return details, nil
}

// normalizeSlots fills in IDs/positions and rejects duplicates.
func normalizeSlots(slots core.SlotList) error {
	seen := map[string]bool{}
	for i := range slots {
		if slots[i].ID == "" {
			slots[i].ID = core.Slugify(slots[i].Name)
		}
		if slots[i].ID == "" {
			return fmt.Errorf("%w: slot %d needs a name or id", core.ErrInvalid, i)
		}
		if seen[slots[i].ID] {
			return fmt.Errorf("%w: duplicate slot id %q", core.ErrInvalid, slots[i].ID)
		}
		seen[slots[i].ID] = true

		if slots[i].Name == "" {
			slots[i].Name = slots[i].ID
		}
		if slots[i].Position == 0 {
			slots[i].Position = i
		}
		if len(slots[i].AcceptedCategories) == 0 {
			slots[i].AcceptedCategories = []string{"universal"}
		}
	}
	return nil
}

func defaultString(v, fallback string) string {
	if strings.TrimSpace(v) == "" {
		return fallback
	}
	return v
}

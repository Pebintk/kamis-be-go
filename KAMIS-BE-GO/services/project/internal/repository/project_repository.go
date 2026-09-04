// Package repository is the data-access layer (the Spring Data JPA repositories).
package repository

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/karina/kamis-be-go/pkg/database"
	"github.com/karina/kamis-be-go/services/project/internal/model"
	"gorm.io/gorm"
)

type ProjectRepository struct{ db *gorm.DB }

func NewProjectRepository(db *gorm.DB) *ProjectRepository { return &ProjectRepository{db: db} }

// Filter carries the optional list predicates.
type Filter struct {
	// IDSearch and ProjectName are one search box: a row matches if either
	// substring hits. See the note in query about what Java did here.
	IDSearch    string
	ProjectName string

	ProjectStatus   *int
	ProjectType     *bool
	ProjectClientID string
	StartDate       *time.Time
	EndDate         *time.Time
	StartNominal    *int64
	EndNominal      *int64
}

func (r *ProjectRepository) query(ctx context.Context, f Filter) *gorm.DB {
	q := r.db.WithContext(ctx).Model(&model.Project{})

	// The id and name boxes are a single search: match either.
	//
	// The legacy SQL wrote this as
	//   (:idSearch IS NULL OR id LIKE …) OR (:projectName IS NULL OR name LIKE …)
	// which collapses to TRUE whenever only one of the two is supplied — so
	// searching by id alone, or by name alone, silently matched every project.
	// Only supplying both filtered anything. Fixed: whichever boxes are filled
	// are OR'd together, and an empty box contributes nothing.
	switch {
	case f.IDSearch != "" && f.ProjectName != "":
		q = q.Where("id ILIKE ? OR project_name ILIKE ?", "%"+f.IDSearch+"%", "%"+f.ProjectName+"%")
	case f.IDSearch != "":
		q = q.Where("id ILIKE ?", "%"+f.IDSearch+"%")
	case f.ProjectName != "":
		q = q.Where("project_name ILIKE ?", "%"+f.ProjectName+"%")
	}

	if f.ProjectStatus != nil {
		q = q.Where("project_status = ?", *f.ProjectStatus)
	}
	if f.ProjectType != nil {
		q = q.Where("project_type = ?", *f.ProjectType)
	}
	if f.ProjectClientID != "" {
		q = q.Where("project_client_id ILIKE ?", "%"+f.ProjectClientID+"%")
	}
	if f.StartDate != nil {
		q = q.Where("project_start_date >= ?", *f.StartDate)
	}
	if f.EndDate != nil {
		q = q.Where("project_end_date <= ?", *f.EndDate)
	}
	if f.StartNominal != nil {
		q = q.Where("project_total_pemasukkan >= ?", *f.StartNominal)
	}
	if f.EndNominal != nil {
		q = q.Where("project_total_pemasukkan <= ?", *f.EndNominal)
	}
	return q
}

// FindFiltered returns every matching project, oldest start date first.
func (r *ProjectRepository) FindFiltered(ctx context.Context, f Filter) ([]model.Project, error) {
	var out []model.Project
	err := r.query(ctx, f).Order("project_start_date ASC").Find(&out).Error
	return out, err
}

// FindFilteredPaginated returns one page of matching projects plus the total.
func (r *ProjectRepository) FindFilteredPaginated(ctx context.Context, f Filter, number, size int) ([]model.Project, int64, error) {
	q := r.query(ctx, f)

	var total int64
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	var out []model.Project
	if err := q.Order("project_start_date ASC").Offset(number * size).Limit(size).Find(&out).Error; err != nil {
		return nil, 0, err
	}
	return out, total, nil
}

func (r *ProjectRepository) FindByID(ctx context.Context, id string) (*model.Project, error) {
	var p model.Project
	if err := r.db.WithContext(ctx).Where("id = ?", id).First(&p).Error; err != nil {
		return nil, database.Translate(err)
	}
	return &p, nil
}

func (r *ProjectRepository) Save(ctx context.Context, p *model.Project) error {
	return database.Translate(r.db.WithContext(ctx).Save(p).Error)
}

// ---- id generation ----

// maxIDAttempts bounds the retry below.
const maxIDAttempts = 5

// idPrefix is the D (distribution) or P (sale) a project id opens with.
func idPrefix(projectType bool) string {
	if projectType == model.TypePengiriman {
		return "D"
	}
	return "P"
}

// CreateWithGeneratedID inserts a project with its usages and first log entry in
// one transaction.
//
// The id is {D|P}{NNN}{yymmdd} — e.g. D001260903 — where NNN counts the projects
// created today. Java read that count in a separate query before the insert, so
// two concurrent creates produced the same id and `save()` on an existing
// primary key is an update: one project silently overwrote the other. The count
// happens inside the transaction here, and a colliding insert is retried.
func (r *ProjectRepository) CreateWithGeneratedID(
	ctx context.Context,
	p *model.Project,
	assets []model.ProjectAssetUsage,
	resources []model.ProjectResourceUsage,
	logUsername string,
	actionFor func(projectID string) string,
) error {
	now := time.Now()
	suffix := now.Format("060102")
	dayStart := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())

	var lastErr error
	for attempt := 0; attempt < maxIDAttempts; attempt++ {
		err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
			var todayCount int64
			if err := tx.Model(&model.Project{}).
				Where("created_date >= ? AND created_date < ?", dayStart, dayStart.AddDate(0, 0, 1)).
				Count(&todayCount).Error; err != nil {
				return err
			}

			p.ID = fmt.Sprintf("%s%03d%s", idPrefix(p.ProjectType), todayCount+1+int64(attempt), suffix)
			if err := tx.Create(p).Error; err != nil {
				return database.Translate(err)
			}

			for i := range assets {
				assets[i].ProjectID = p.ID
			}
			if len(assets) > 0 {
				if err := tx.Create(&assets).Error; err != nil {
					return err
				}
			}

			for i := range resources {
				resources[i].ProjectID = p.ID
			}
			if len(resources) > 0 {
				if err := tx.Create(&resources).Error; err != nil {
					return err
				}
			}

			return tx.Create(&model.LogProject{
				ProjectID: p.ID,
				Username:  logUsername,
				Action:    actionFor(p.ID),
			}).Error
		})
		if err == nil {
			return nil
		}
		lastErr = err
		if !errors.Is(err, database.ErrDuplicate) {
			return err
		}
	}
	return fmt.Errorf("could not allocate a project id after %d attempts: %w", maxIDAttempts, lastErr)
}

// ---- usages and logs ----

// FindAssetUsageFor loads the vehicles booked to several projects at once.
func (r *ProjectRepository) FindAssetUsageFor(ctx context.Context, projectIDs []string) (map[string][]model.ProjectAssetUsage, error) {
	out := make(map[string][]model.ProjectAssetUsage, len(projectIDs))
	if len(projectIDs) == 0 {
		return out, nil
	}

	var rows []model.ProjectAssetUsage
	if err := r.db.WithContext(ctx).Where("project_id IN ?", projectIDs).
		Order("plat_nomor").Find(&rows).Error; err != nil {
		return nil, err
	}
	for _, row := range rows {
		out[row.ProjectID] = append(out[row.ProjectID], row)
	}
	return out, nil
}

// FindResourceUsageFor loads the catalogue items consumed by several projects.
func (r *ProjectRepository) FindResourceUsageFor(ctx context.Context, projectIDs []string) (map[string][]model.ProjectResourceUsage, error) {
	out := make(map[string][]model.ProjectResourceUsage, len(projectIDs))
	if len(projectIDs) == 0 {
		return out, nil
	}

	var rows []model.ProjectResourceUsage
	if err := r.db.WithContext(ctx).Where("project_id IN ?", projectIDs).
		Order("resource_id").Find(&rows).Error; err != nil {
		return nil, err
	}
	for _, row := range rows {
		out[row.ProjectID] = append(out[row.ProjectID], row)
	}
	return out, nil
}

// FindLogsFor loads the audit trails of several projects, oldest first.
func (r *ProjectRepository) FindLogsFor(ctx context.Context, projectIDs []string) (map[string][]model.LogProject, error) {
	out := make(map[string][]model.LogProject, len(projectIDs))
	if len(projectIDs) == 0 {
		return out, nil
	}

	var rows []model.LogProject
	if err := r.db.WithContext(ctx).Where("project_id IN ?", projectIDs).
		Order("action_date").Find(&rows).Error; err != nil {
		return nil, err
	}
	for _, row := range rows {
		out[row.ProjectID] = append(out[row.ProjectID], row)
	}
	return out, nil
}

// ReplaceAssetUsage swaps a project's vehicle bookings for a new set.
func (r *ProjectRepository) ReplaceAssetUsage(ctx context.Context, projectID string, usages []model.ProjectAssetUsage) error {
	return database.Translate(r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("project_id = ?", projectID).Delete(&model.ProjectAssetUsage{}).Error; err != nil {
			return err
		}
		if len(usages) == 0 {
			return nil
		}
		for i := range usages {
			usages[i].ProjectID = projectID
		}
		return tx.Create(&usages).Error
	}))
}

// ReplaceResourceUsage swaps a project's consumed catalogue items for a new set.
func (r *ProjectRepository) ReplaceResourceUsage(ctx context.Context, projectID string, usages []model.ProjectResourceUsage) error {
	return database.Translate(r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("project_id = ?", projectID).Delete(&model.ProjectResourceUsage{}).Error; err != nil {
			return err
		}
		if len(usages) == 0 {
			return nil
		}
		for i := range usages {
			usages[i].ProjectID = projectID
		}
		return tx.Create(&usages).Error
	}))
}

func (r *ProjectRepository) AppendLog(ctx context.Context, entry *model.LogProject) error {
	return database.Translate(r.db.WithContext(ctx).Create(entry).Error)
}

// Delete removes a project and everything hanging off it. It exists to undo a
// creation whose external side effects failed, so a half-made project is never
// left behind.
func (r *ProjectRepository) Delete(ctx context.Context, projectID string) error {
	return database.Translate(r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		for _, child := range []any{
			&model.ProjectAssetUsage{}, &model.ProjectResourceUsage{}, &model.LogProject{},
		} {
			if err := tx.Where("project_id = ?", projectID).Delete(child).Error; err != nil {
				return err
			}
		}
		return tx.Where("id = ?", projectID).Delete(&model.Project{}).Error
	}))
}

// ---- reporting aggregations ----
//
// Every chart and summary counts by createdDate — when the project was recorded
// — not by when it runs, which is what the legacy queries grouped on.

// PeriodCount is one row of a grouped count.
type PeriodCount struct {
	Period string
	Count  int64
}

// DayCount is one row of a per-day grouped count, kept as a date so the caller
// can bucket it into weeks.
type DayCount struct {
	Day   time.Time
	Count int64
}

// StatusScope narrows an aggregation to, or away from, a set of statuses.
type StatusScope struct {
	Statuses []int
	// Exclude counts everything *but* Statuses — the chart's "ALL" filter uses
	// this to mean "every project that was not cancelled".
	Exclude bool
}

func (r *ProjectRepository) window(ctx context.Context, start, end time.Time, projectType bool, scope StatusScope) *gorm.DB {
	q := r.db.WithContext(ctx).Model(&model.Project{}).
		Where("created_date >= ? AND created_date <= ?", start, end).
		Where("project_type = ?", projectType)

	if len(scope.Statuses) == 0 {
		return q
	}
	if scope.Exclude {
		return q.Where("project_status NOT IN ?", scope.Statuses)
	}
	return q.Where("project_status IN ?", scope.Statuses)
}

func (r *ProjectRepository) countByExpr(ctx context.Context, expr string, start, end time.Time, projectType bool, scope StatusScope) ([]PeriodCount, error) {
	var out []PeriodCount
	err := r.window(ctx, start, end, projectType, scope).
		Select(expr + " AS period, COUNT(*) AS count").
		Group(expr).Order("period").Scan(&out).Error
	return out, err
}

// CountByMonth groups as yyyy-MM.
func (r *ProjectRepository) CountByMonth(ctx context.Context, start, end time.Time, projectType bool, scope StatusScope) ([]PeriodCount, error) {
	return r.countByExpr(ctx, "to_char(created_date, 'YYYY-MM')", start, end, projectType, scope)
}

// CountByQuarter groups as yyyy-Qn.
func (r *ProjectRepository) CountByQuarter(ctx context.Context, start, end time.Time, projectType bool, scope StatusScope) ([]PeriodCount, error) {
	return r.countByExpr(ctx,
		"to_char(created_date, 'YYYY') || '-Q' || to_char(created_date, 'Q')",
		start, end, projectType, scope)
}

// CountByYear groups as yyyy.
func (r *ProjectRepository) CountByYear(ctx context.Context, start, end time.Time, projectType bool, scope StatusScope) ([]PeriodCount, error) {
	return r.countByExpr(ctx, "to_char(created_date, 'YYYY')", start, end, projectType, scope)
}

// CountByDay groups by calendar day, which the service then buckets into weeks.
//
// The legacy query grouped by the raw timestamp column, so every project landed
// in its own group; the in-memory re-aggregation into weeks happened to hide it.
func (r *ProjectRepository) CountByDay(ctx context.Context, start, end time.Time, projectType bool, scope StatusScope) ([]DayCount, error) {
	var out []DayCount
	err := r.window(ctx, start, end, projectType, scope).
		Select("date_trunc('day', created_date) AS day, COUNT(*) AS count").
		Group("date_trunc('day', created_date)").Order("day").Scan(&out).Error
	return out, err
}

// CountByType counts the projects of one kind recorded in a window, whatever
// their status.
func (r *ProjectRepository) CountByType(ctx context.Context, start, end time.Time, projectType bool) (int64, error) {
	var count int64
	err := r.window(ctx, start, end, projectType, StatusScope{}).Count(&count).Error
	return count, err
}

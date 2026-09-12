// Package app 刷新编排，对应 Android 版 AppViewModel.kt 的数据层职责
// （CLI 无持续 UI，单次执行：刷新 → 渲染 → 退出，故不保留旧数据）。
package app

import (
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/xieguiawu/llm-api-check/internal/config"
	"github.com/xieguiawu/llm-api-check/internal/models"
	"github.com/xieguiawu/llm-api-check/internal/parsers"
	"github.com/xieguiawu/llm-api-check/internal/repo"
)

// DeepSeekResult 单个 DeepSeek 账号的刷新结果（对应 DeepSeekUi）。
// Error 合并 balance/cost 两者错误（"\n" 连接，无错误则空）。
type DeepSeekResult struct {
	Account models.DeepSeekAccount  `json:"account"`
	Balance *models.DeepSeekBalance `json:"balance,omitempty"`
	Cost    *models.DeepSeekCost    `json:"cost,omitempty"`
	Error   string                  `json:"error,omitempty"`
}

// AccountResult 单个 OpenCode 账号的刷新结果（对应 AccountUi）。
type AccountResult struct {
	Account    models.Account     `json:"account"`
	GoUsage    *models.GoUsage    `json:"go_usage,omitempty"`
	ZenBilling *models.ZenBilling `json:"zen_billing,omitempty"`
	Error      string             `json:"error,omitempty"`
}

// QwenResult 单个 Qwen 账号的刷新结果（对应 QwenUi）。
// Plan 走 API Key（模型清单）；Usage 走 Bailian CLI 或控制台 Cookie（配额窗口）；
// Stats 走 Bailian CLI（用量分析，--stats 时才拉）。
type QwenResult struct {
	Account models.QwenAccount  `json:"account"`
	Plan    *models.QwenPlan    `json:"plan,omitempty"`
	Usage   *models.QwenUsage   `json:"usage,omitempty"`
	Stats   *models.QwenSummary `json:"stats,omitempty"`
	Error   string              `json:"error,omitempty"`
	// CLIEnabled 本机是否探测到 Bailian CLI（渲染层区分“暂无数据”与“未配置凭据”）
	CLIEnabled bool `json:"-"`
	// CLILoginCmd / CLIInstallCmd 可直接粘贴的登录与安装命令（含真实路径，
	// 不用裸 bailian——默认独立 prefix 安装不在 PATH）
	CLILoginCmd   string `json:"-"`
	CLIInstallCmd string `json:"-"`
}

// BaiResult 单个白B.AI 账号的刷新结果。Plan = 模型清单（推理面），
// Points = 积分额度（控制台 tRPC）；两路独立，任一路成功即有数据可显示。
type BaiResult struct {
	Account models.BaiAccount     `json:"account"`
	Plan    *models.BaiPlan       `json:"plan,omitempty"`
	Points  *models.BaiPoints     `json:"points,omitempty"`
	Stats   *models.BaiUsageStats `json:"stats,omitempty"`
	Error   string                `json:"error,omitempty"`
}

// GptzeroResult 单个 GPTZero 账号的刷新结果。单端点（/v2/users/me），Usage 即全部数据。
type GptzeroResult struct {
	Account models.GptzeroAccount `json:"account"`
	Usage   *models.GptzeroUsage  `json:"usage,omitempty"`
	Error   string                `json:"error,omitempty"`
}

// LongCatResult 单个 LongCat 账号的刷新结果。Plan = 模型清单（/v1/models）；
// Usage = 余额探活（小额 chat 探测 402/200）。
// LongCat 无公开配额 API，BalanceOK 只反映探活时点的余额是否 >0。
type LongCatResult struct {
	Account models.LongCatAccount `json:"account"`
	Plan    *models.LongCatPlan   `json:"plan,omitempty"`
	Usage   *models.LongCatUsage  `json:"usage,omitempty"`
	Error   string                `json:"error,omitempty"`
}

// GalaxyResult 单个智星云账号的刷新结果（对应 GalaxyUi）。
// Balance 必需；Status/Instances/Cost 任一失败只影响该段（错误合并进 Error）。
type GalaxyResult struct {
	Account   models.GalaxyAccount      `json:"account"`
	Balance   *models.GalaxyBalance     `json:"balance,omitempty"`
	Status    *models.GalaxyStatusCount `json:"status,omitempty"`
	Instances []models.GalaxyInstance   `json:"instances,omitempty"`
	Cost      *models.GalaxyCost        `json:"cost,omitempty"`
	Error     string                    `json:"error,omitempty"`
}

// HourlyCost 运行中实例的合计时价（元/时）——余额还能撑多久算得出来
func (r GalaxyResult) HourlyCost() float64 {
	var sum float64
	for _, in := range r.Instances {
		if in.Status == 1 || in.Status == 4 || in.Status == 5 {
			sum += in.TotalCost
		}
	}
	return sum
}

// Result 全量刷新结果
type Result struct {
	DeepSeek    []DeepSeekResult
	Accounts    []AccountResult
	Qwen        []QwenResult
	Galaxy      []GalaxyResult
	Bai         []BaiResult
	Gptzero     []GptzeroResult
	LongCat     []LongCatResult
	LastUpdated time.Time
}

// Repos 仓库注入点（测试可替换为 httptest 服务）
type Repos struct {
	DeepSeek *repo.DeepSeekRepo
	OpenCode *repo.OpenCodeRepo
	Qwen     *repo.QwenRepo
	Galaxy   *repo.GalaxyRepo
	Bai      *repo.BaiRepo
	Gptzero  *repo.GptzeroRepo
	LongCat  *repo.LongCatRepo
}

// GalaxyInstanceLimit 单次刷新展示的活跃实例上限（防止大账号拉穿）
const GalaxyInstanceLimit = 20

// App 刷新编排器（对应 AppViewModel）
type App struct {
	Repos *Repos
	Cfg   *config.Config

	mu         sync.Mutex
	refreshing bool
}

// New 默认构造：真实端点 + 15s 超时 client
func New(cfg *config.Config) *App {
	return &App{
		Repos: &Repos{
			DeepSeek: repo.NewDeepSeekRepo(),
			OpenCode: repo.NewOpenCodeRepo(),
			Qwen:     repo.NewQwenRepo(),
			Galaxy:   repo.NewGalaxyRepo(),
			Bai:      repo.NewBaiRepo(),
			Gptzero:  repo.NewGptzeroRepo(),
			LongCat:  repo.NewLongCatRepo(),
		},
		Cfg: cfg,
	}
}

// NewWithRepos 测试注入仓库
func NewWithRepos(cfg *config.Config, repos *Repos) *App {
	return &App{Repos: repos, Cfg: cfg}
}

// RefreshAll 刷新全部（DeepSeek 全账号 + OpenCode 全账号，并行）。
// 重入保护：刷新进行中再次调用直接忽略（对应 Android 版 refreshing 判断）。
// 以配置为准重建账号列表；成功后写入 Config.LastUpdate["all"]。
func (a *App) RefreshAll() (Result, error) {
	a.mu.Lock()
	if a.refreshing {
		a.mu.Unlock()
		return Result{}, nil
	}
	a.refreshing = true
	a.mu.Unlock()
	defer func() {
		a.mu.Lock()
		a.refreshing = false
		a.mu.Unlock()
	}()

	dsAccounts := append([]models.DeepSeekAccount(nil), a.Cfg.DeepSeekAccounts...)
	accounts := append([]models.Account(nil), a.Cfg.Accounts...)
	qwenAccounts := append([]models.QwenAccount(nil), a.Cfg.QwenAccounts...)
	galaxyAccounts := append([]models.GalaxyAccount(nil), a.Cfg.GalaxyAccounts...)
	baiAccounts := append([]models.BaiAccount(nil), a.Cfg.BaiAccounts...)
	gzAccounts := append([]models.GptzeroAccount(nil), a.Cfg.GptzeroAccounts...)
	lcAccounts := append([]models.LongCatAccount(nil), a.Cfg.LongCatAccounts...)
	dsRes := make([]DeepSeekResult, len(dsAccounts))
	accRes := make([]AccountResult, len(accounts))
	qwenRes := make([]QwenResult, len(qwenAccounts))
	galaxyRes := make([]GalaxyResult, len(galaxyAccounts))
	baiRes := make([]BaiResult, len(baiAccounts))
	gzRes := make([]GptzeroResult, len(gzAccounts))
	lcRes := make([]LongCatResult, len(lcAccounts))
	var wg sync.WaitGroup
	for i, acc := range dsAccounts {
		wg.Add(1)
		go func(i int, acc models.DeepSeekAccount) {
			defer wg.Done()
			dsRes[i] = a.refreshDeepSeek(acc)
		}(i, acc)
	}
	for i, acc := range accounts {
		wg.Add(1)
		go func(i int, acc models.Account) {
			defer wg.Done()
			accRes[i] = a.refreshAccount(acc)
		}(i, acc)
	}
	for i, acc := range qwenAccounts {
		wg.Add(1)
		go func(i int, acc models.QwenAccount) {
			defer wg.Done()
			qwenRes[i] = a.refreshQwen(acc)
		}(i, acc)
	}
	for i, acc := range galaxyAccounts {
		wg.Add(1)
		go func(i int, acc models.GalaxyAccount) {
			defer wg.Done()
			galaxyRes[i] = a.refreshGalaxy(acc, GalaxyInstanceLimit)
		}(i, acc)
	}
	for i, acc := range baiAccounts {
		wg.Add(1)
		go func(i int, acc models.BaiAccount) {
			defer wg.Done()
			baiRes[i] = a.refreshBai(acc)
		}(i, acc)
	}
	for i, acc := range gzAccounts {
		wg.Add(1)
		go func(i int, acc models.GptzeroAccount) {
			defer wg.Done()
			gzRes[i] = a.refreshGptzero(acc)
		}(i, acc)
	}
	for i, acc := range lcAccounts {
		wg.Add(1)
		go func(i int, acc models.LongCatAccount) {
			defer wg.Done()
			lcRes[i] = a.refreshLongCat(acc)
		}(i, acc)
	}
	wg.Wait()
	now := time.Now()
	a.Cfg.SetLastUpdate("all", now.UnixMilli())
	return Result{DeepSeek: dsRes, Accounts: accRes, Qwen: qwenRes, Galaxy: galaxyRes, Bai: baiRes, Gptzero: gzRes, LongCat: lcRes, LastUpdated: now}, nil
}

// RefreshDeepSeek 按 id 刷新单个 DeepSeek 账号（对应 refreshDeepSeekNow）
func (a *App) RefreshDeepSeek(id string) (DeepSeekResult, error) {
	var acc models.DeepSeekAccount
	found := false
	for _, x := range a.Cfg.DeepSeekAccounts {
		if x.ID == id {
			acc = x
			found = true
			break
		}
	}
	if !found {
		return DeepSeekResult{}, errors.New("账号不存在或已被删除")
	}
	return a.refreshDeepSeek(acc), nil
}

// refreshDeepSeek 刷余额 + 消费（配置了 token 才拉消费），错误合并。
// 对应 AppViewModel.refreshDeepSeekNow 的 listOfNotNull(...).joinToString("\n") 逻辑。
func (a *App) refreshDeepSeek(acc models.DeepSeekAccount) DeepSeekResult {
	res := DeepSeekResult{Account: acc}
	bal, balErr := a.Repos.DeepSeek.Balance(acc.ApiKey)
	if balErr == nil {
		res.Balance = &bal
	}
	if acc.HasToken() {
		cost, costErr := a.Repos.DeepSeek.Cost(acc.PlatformToken)
		if costErr == nil {
			res.Cost = &cost
		}
		res.Error = joinErrors(balErr, costErr)
	} else {
		res.Error = errMsg(balErr)
	}
	return res
}

// RefreshAccount 按 id 刷新单个 OpenCode 账号（对应 refreshAccountNow）
func (a *App) RefreshAccount(id string) (AccountResult, error) {
	var acc models.Account
	found := false
	for _, x := range a.Cfg.Accounts {
		if x.ID == id {
			acc = x
			found = true
			break
		}
	}
	if !found {
		return AccountResult{}, errors.New("账号不存在或已被删除")
	}
	return a.refreshAccount(acc), nil
}

// refreshAccount 刷 Go usage + Zen billing（配置了 workspace/cookie 才拉 Zen）。
func (a *App) refreshAccount(acc models.Account) AccountResult {
	res := AccountResult{Account: acc}
	goU, goErr := a.Repos.OpenCode.GoUsage(acc)
	if goErr == nil {
		res.GoUsage = &goU
	}
	if acc.HasZen() {
		zen, zenErr := a.Repos.OpenCode.ZenBilling(acc)
		if zenErr == nil {
			res.ZenBilling = &zen
		}
		res.Error = joinErrors(goErr, zenErr)
	} else {
		res.Error = errMsg(goErr)
	}
	return res
}

// RefreshQwen 按 id 刷新单个 Qwen 账号（对应 refreshQwenNow）
func (a *App) RefreshQwen(id string) (QwenResult, error) {
	var acc models.QwenAccount
	found := false
	for _, x := range a.Cfg.QwenAccounts {
		if x.ID == id {
			acc = x
			found = true
			break
		}
	}
	if !found {
		return QwenResult{}, errors.New("账号不存在或已被删除")
	}
	return a.refreshQwen(acc), nil
}

// RefreshQwenStats 拉用量分析（token 统计 + 免费额度，仅 Bailian CLI 通道）。
func (a *App) RefreshQwenStats(id string) (QwenResult, error) {
	var acc models.QwenAccount
	found := false
	for _, x := range a.Cfg.QwenAccounts {
		if x.ID == id {
			acc = x
			found = true
			break
		}
	}
	if !found {
		return QwenResult{}, errors.New("账号不存在或已被删除")
	}
	if !a.Repos.Qwen.CLIEnabled() {
		return QwenResult{}, fmt.Errorf("用量分析需要 Bailian CLI：本机未探测到。安装：%s；登录：%s",
			a.Repos.Qwen.CLIInstallCmd(), a.Repos.Qwen.CLILoginCmd())
	}
	s, err := a.Repos.Qwen.CLI.Summary(acc)
	if err != nil {
		return QwenResult{}, err
	}
	return QwenResult{Account: acc, Stats: &s}, nil
}

// RefreshQwen 刷模型清单（API Key）+ 配额窗口（Bailian CLI 或 Cookie 任一可用时拉取），错误合并。
func (a *App) refreshQwen(acc models.QwenAccount) QwenResult {
	res := QwenResult{Account: acc,
		CLIEnabled:    a.Repos.Qwen.CLIEnabled(),
		CLILoginCmd:   a.Repos.Qwen.CLILoginCmd(),
		CLIInstallCmd: a.Repos.Qwen.CLIInstallCmd(),
	}
	plan, planErr := a.Repos.Qwen.Plan(acc)
	if planErr == nil {
		res.Plan = &plan
	}
	if acc.HasCookie() || a.Repos.Qwen.CLIEnabled() {
		usage, usageErr := a.Repos.Qwen.Usage(acc)
		if usageErr == nil {
			res.Usage = &usage
		}
		res.Error = joinErrors(planErr, usageErr)
	} else {
		res.Error = errMsg(planErr)
	}
	return res
}

// RefreshGalaxy 按 id 刷新单个智星云账号（对应 refreshGalaxyNow）。
// limit ≤0 表示实例列表不限量（受仓库翻页上限约束）。
func (a *App) RefreshGalaxy(id string, limit int) (GalaxyResult, error) {
	var acc models.GalaxyAccount
	found := false
	for _, x := range a.Cfg.GalaxyAccounts {
		if x.ID == id {
			acc = x
			found = true
			break
		}
	}
	if !found {
		return GalaxyResult{}, errors.New("账号不存在或已被删除")
	}
	return a.refreshGalaxy(acc, limit), nil
}

// refreshGalaxy 并发拉四类数据（余额/统计/实例/消耗）。
// 任一路失败不中断其余路：余额拉到就显示余额（同 DeepSeek balance/cost 的处理），
// 全部失败时上层据「无数据 + 有错误」判 exit 1。
func (a *App) refreshGalaxy(acc models.GalaxyAccount, limit int) GalaxyResult {
	res := GalaxyResult{Account: acc}
	if a.Repos == nil || a.Repos.Galaxy == nil {
		res.Error = "智星云仓库未初始化"
		return res
	}
	var (
		wg      sync.WaitGroup
		bal     models.GalaxyBalance
		balErr  error
		cnt     models.GalaxyStatusCount
		cntErr  error
		insts   []models.GalaxyInstance
		insErr  error
		cost    models.GalaxyCost
		costErr error
	)
	fetch := func(fn func()) {
		wg.Add(1)
		go func() {
			defer wg.Done()
			fn()
		}()
	}
	fetch(func() { bal, balErr = a.Repos.Galaxy.Balance(acc) })
	fetch(func() { cnt, cntErr = a.Repos.Galaxy.StatusCount(acc) })
	fetch(func() { insts, insErr = a.Repos.Galaxy.Instances(acc, repo.GalaxyStatusDefault, limit) })
	fetch(func() { cost, costErr = a.Repos.Galaxy.Cost(acc) })
	wg.Wait()

	if balErr == nil {
		res.Balance = &bal
	}
	if cntErr == nil {
		res.Status = &cnt
	}
	if insErr == nil {
		res.Instances = insts
	}
	if costErr == nil {
		res.Cost = &cost
	}
	res.Error = joinErrors(balErr, cntErr, insErr, costErr)
	return res
}

// RefreshBaiStats 拉用量分析（usage.records 分页聚合，--stats 时才调）。
// 对齐 RefreshQwenStats：stats 失败不影响详情主体，错误由调用方 join 进结果。
func (a *App) RefreshBaiStats(id string) (BaiResult, error) {
	var acc models.BaiAccount
	found := false
	for _, x := range a.Cfg.BaiAccounts {
		if x.ID == id {
			acc = x
			found = true
			break
		}
	}
	if !found {
		return BaiResult{}, errors.New("账号不存在或已被删除")
	}
	if a.Repos == nil || a.Repos.Bai == nil {
		return BaiResult{}, errors.New("BAI 仓库未初始化")
	}
	s, err := a.Repos.Bai.Stats(acc.ApiKey)
	if err != nil {
		return BaiResult{}, err
	}
	return BaiResult{Account: acc, Stats: &s}, nil
}

// RefreshBai 按 id 刷新单个白B.AI 账号（模型清单 + 积分额度，同一把 API Key）。
func (a *App) RefreshBai(id string) (BaiResult, error) {
	var acc models.BaiAccount
	found := false
	for _, x := range a.Cfg.BaiAccounts {
		if x.ID == id {
			acc = x
			found = true
			break
		}
	}
	if !found {
		return BaiResult{}, errors.New("账号不存在或已被删除")
	}
	return a.refreshBai(acc), nil
}

// refreshBai 并发拉两路：模型清单（api.b.ai）+ 积分额度（chat.b.ai）。
// 两路不同主机、同一把 key：一路失败不抖掉另一路已拿到的数据（同 refreshGalaxy）。
func (a *App) refreshBai(acc models.BaiAccount) BaiResult {
	res := BaiResult{Account: acc}
	if a.Repos == nil || a.Repos.Bai == nil {
		res.Error = "BAI 仓库未初始化"
		return res
	}
	var (
		wg      sync.WaitGroup
		plan    models.BaiPlan
		planErr error
		pts     models.BaiPoints
		ptsErr  error
	)
	wg.Add(2)
	go func() {
		defer wg.Done()
		plan, planErr = a.Repos.Bai.Models(acc.ApiKey)
		if planErr != nil {
			return
		}
		// 免费通道运行时探活：清单「在」≠ 可用（503 间歇故障只有真发推理才查得出）。
		// 只探清单内存在的盯梢模型，缺失项不浪费请求；探活失败不抖掉清单。
		var present []string
		missing := plan.MissingFreeFlash()
		for _, want := range models.BaiFreeFlashModels {
			skip := false
			for _, m := range missing {
				if m == want {
					skip = true
					break
				}
			}
			if !skip {
				present = append(present, want)
			}
		}
		if probes, err := a.Repos.Bai.ProbeFreeFlash(acc.ApiKey, present); err == nil {
			plan.Probes = probes
		}
	}()
	go func() {
		defer wg.Done()
		pts, ptsErr = a.Repos.Bai.Points(acc.ApiKey)
	}()
	wg.Wait()
	if planErr == nil {
		res.Plan = &plan
	}
	if ptsErr == nil {
		res.Points = &pts
	}
	res.Error = joinErrors(planErr, ptsErr)
	return res
}

// RefreshGptzero 按 id 刷新单个 GPTZero 账号（单端点额度查询）。
func (a *App) RefreshGptzero(id string) (GptzeroResult, error) {
	var acc models.GptzeroAccount
	found := false
	for _, x := range a.Cfg.GptzeroAccounts {
		if x.ID == id {
			acc = x
			found = true
			break
		}
	}
	if !found {
		return GptzeroResult{}, errors.New("账号不存在或已被删除")
	}
	return a.refreshGptzero(acc), nil
}

// refreshGptzero 拉取账号与月度额度。
func (a *App) refreshGptzero(acc models.GptzeroAccount) GptzeroResult {
	res := GptzeroResult{Account: acc}
	if a.Repos == nil || a.Repos.Gptzero == nil {
		res.Error = "GPTZero 仓库未初始化"
		return res
	}
	u, err := a.Repos.Gptzero.Usage(acc.ApiKey)
	if err != nil {
		res.Error = err.Error()
		return res
	}
	res.Usage = &u
	return res
}

// RefreshLongCat 按 id 刷新单个 LongCat 账号（模型清单 + 余额探活）。
func (a *App) RefreshLongCat(id string) (LongCatResult, error) {
	var acc models.LongCatAccount
	found := false
	for _, x := range a.Cfg.LongCatAccounts {
		if x.ID == id {
			acc = x
			found = true
			break
		}
	}
	if !found {
		return LongCatResult{}, errors.New("账号不存在或已被删除")
	}
	return a.refreshLongCat(acc), nil
}

// refreshLongCat 并发拉两路：模型清单（/v1/models）+ 余额探活（小额 chat）。
// 两路独立：清单失败不抖掉探活结果，探活失败不抖掉清单。
func (a *App) refreshLongCat(acc models.LongCatAccount) LongCatResult {
	res := LongCatResult{Account: acc}
	if a.Repos == nil || a.Repos.LongCat == nil {
		res.Error = "LongCat 仓库未初始化"
		return res
	}
	var (
		wg      sync.WaitGroup
		plan    models.LongCatPlan
		planErr error
		balOK   *bool
		errorr  error
	)
	wg.Add(2)
	go func() {
		defer wg.Done()
		plan, planErr = a.Repos.LongCat.Models(acc.ApiKey)
	}()
	go func() {
		defer wg.Done()
		ok, err := a.Repos.LongCat.ProbeBalance(acc.ApiKey)
		if err == nil {
			balOK = &ok
		} else {
			// 401 认证错误向上传递，其他错误只记录不致命
			if errors.Is(err, parsers.ErrLongCatAuth) {
				errorr = err
			}
		}
	}()
	wg.Wait()
	if planErr == nil {
		res.Plan = &plan
	}
	if balOK != nil || errorr != nil {
		res.Usage = &models.LongCatUsage{Models: plan.Models}
		if balOK != nil {
			res.Usage.BalanceOK = balOK
		}
	}
	// 错误合并：认证错误最优先，清单错误次之
	if errorr != nil {
		res.Error = errorr.Error()
	} else if planErr != nil {
		res.Error = planErr.Error()
	}
	return res
}

// LastUpdated 从配置读取最近一次全量刷新时间（对应 SecureSettings.lastUpdate("all")）
func (a *App) LastUpdated() time.Time {
	if a.Cfg == nil {
		return time.Time{}
	}
	return a.Cfg.LastUpdateAt("all")
}

// joinErrors 合并错误消息（"\n" 连接，全部 nil → 空串）。
// 同文本去重：多路共用一把凭据时，凭据错误会逐路重复（同 main.go joinText 的教训）。
func joinErrors(errs ...error) string {
	var msgs []string
	seen := map[string]bool{}
	for _, e := range errs {
		if e == nil {
			continue
		}
		m := e.Error()
		if seen[m] {
			continue
		}
		seen[m] = true
		msgs = append(msgs, m)
	}
	return strings.Join(msgs, "\n")
}

func errMsg(e error) string {
	if e == nil {
		return ""
	}
	return e.Error()
}

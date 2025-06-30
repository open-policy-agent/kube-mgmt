package dynamicdata

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"sort"
	"sync"

	"github.com/open-policy-agent/kube-mgmt/pkg/data"
	"github.com/open-policy-agent/kube-mgmt/pkg/opa"
	"github.com/open-policy-agent/kube-mgmt/pkg/types"

	//lint:ignore SA1019 using OPA v0.x to ensure backwards compatible with pre-1.0 bundles
	"github.com/open-policy-agent/opa/ast"

	//lint:ignore SA1019 using OPA v0.x to ensure backwards compatible with pre-1.0 bundles
	"github.com/open-policy-agent/opa/dependencies"

	//lint:ignore SA1019 using OPA v0.x to ensure backwards compatible with pre-1.0 bundles
	"github.com/open-policy-agent/opa/logging"

	//lint:ignore SA1019 using OPA v0.x to ensure backwards compatible with pre-1.0 bundles
	"github.com/open-policy-agent/opa/plugins"

	//lint:ignore SA1019 using OPA v0.x to ensure backwards compatible with pre-1.0 bundles
	"github.com/open-policy-agent/opa/sdk"

	//lint:ignore SA1019 using OPA v0.x to ensure backwards compatible with pre-1.0 bundles
	"github.com/open-policy-agent/opa/storage"

	//lint:ignore SA1019 using OPA v0.x to ensure backwards compatible with pre-1.0 bundles
	"github.com/open-policy-agent/opa/storage/inmem"

	"github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/rest"
)

type Sync struct {
	opaConfig          []byte
	kubeconfig         *rest.Config
	opaURL, opaAuth    string
	ignoreNs           []string
	analysisEntrypoint string
	replicatePath      string
	logger             logging.Logger
	running            map[types.ResourceType]*cancellableSync
	mu                 sync.Mutex
	ready              bool
}

func New(configFile string, analysisEntrypoint string, opaURL, opaAuth string, ignoreNs []string, replicatePath string, kubeconfig *rest.Config, logger logging.Logger) (*Sync, error) {

	bs, err := os.ReadFile(configFile)
	if err != nil {
		return nil, err
	}

	sync := &Sync{
		opaConfig:          bs,
		kubeconfig:         kubeconfig,
		opaAuth:            opaAuth,
		opaURL:             opaURL,
		ignoreNs:           ignoreNs,
		analysisEntrypoint: analysisEntrypoint,
		replicatePath:      replicatePath,
		logger:             logger,
		running:            make(map[types.ResourceType]*cancellableSync),
	}

	return sync, nil
}

func (s *Sync) Run(ctx context.Context) error {

	s.logger.Debug("Loading kubeconfig for API server")
	client, err := dynamic.NewForConfig(s.kubeconfig)
	if err != nil {
		return err
	}

	s.logger.Debug("Resolving resource names to resource types")
	rts, err := resolveResourceTypes(s.kubeconfig)
	if err != nil {
		return err
	}

	s.logger.Debug("Starting analyzer")
	analyzer, err := newAnalyzer(ctx, s.opaConfig, s.replicatePath, s.analysisEntrypoint, s.logger)
	if err != nil {
		return err
	}

	go s.loop(ctx, analyzer, rts, client)

	return nil
}

func (s *Sync) Ready() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.ready {
		s.logger.Debug("Sync is not ready")
		return false
	}
	for rt, r := range s.running {
		if !r.sync.Ready() {
			s.logger.Debug("Replicator for %v is not ready", rt)
			return false
		}
	}
	return true
}

func (s *Sync) loop(ctx context.Context, a *analyzer, rts map[string]types.ResourceType, client *dynamic.DynamicClient) {
	for {
		s.logger.Debug("Sync waiting for analysis result")
		select {
		case result := <-a.C:
			s.logger.Debug("Sync processing analysis result: %v", result)
			s.processAnalysisResult(ctx, result, rts, client)
		case <-ctx.Done():
			s.logger.Debug("Sync shutting down")
		}
	}
}

func (s *Sync) processAnalysisResult(ctx context.Context, result analysisResult, rts map[string]types.ResourceType, client *dynamic.DynamicClient) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// If any of the refs cannot be mapped to gvk then give up.
	for _, ref := range result.Refs {
		if _, ok := rts[ref.Resource]; !ok {
			logrus.Errorf("Cannot resolve Kubernetes resource %q to group/version/resource for dynamic data replication", ref.Resource)
			s.ready = false
			return
		}
	}

	// Otherwise, create and delete data syncs accordingly.
	s.ready = true
	create := map[types.ResourceType]struct{}{}

	for _, ref := range result.Refs {
		rt := rts[ref.Resource]
		create[rt] = struct{}{}
		if _, ok := s.running[rt]; !ok {
			s.logger.Debug("Starting data replication for %v", rt)
			sync := data.NewFromInterface(client, opa.New(s.opaURL, s.opaAuth).Prefix(s.replicatePath), rt, data.WithIgnoreNamespaces(s.ignoreNs))
			ctx, cancel := context.WithCancel(ctx)
			s.running[rt] = &cancellableSync{cancel: cancel, sync: sync}
			go sync.RunContext(ctx)
		} else {
			s.logger.Debug("Data replication for %v already started", rt)
		}
	}

	for rt, sync := range s.running {
		if _, ok := create[rt]; !ok {
			s.logger.Debug("Stopping replication for %v", rt)
			sync.cancel()
			delete(s.running, rt)
		}
	}
}

func resolveResourceTypes(config *rest.Config) (map[string]types.ResourceType, error) {
	discoveryClient, err := discovery.NewDiscoveryClientForConfig(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create discovery client: %v", err)
	}

	resources, err := discoveryClient.ServerPreferredResources()
	if err != nil {
		return nil, fmt.Errorf("failed to get server preferred resources: %v", err)
	}

	result := map[string]types.ResourceType{}

	for _, r := range resources {
		gv, err := schema.ParseGroupVersion(r.GroupVersion)
		if err != nil {
			return nil, err
		}
		for _, ar := range r.APIResources {
			rt := types.ResourceType{
				Namespaced: ar.Namespaced,
				Resource:   ar.Name,
				Group:      ar.Group,
				Version:    ar.Version,
			}
			if rt.Group == "" {
				rt.Group = gv.Group
			}
			if rt.Version == "" {
				rt.Version = gv.Version
			}
			logrus.Infof("Discovered resource %v mapping to type %v (namespaced: %v)", ar.Name, rt, rt.Namespaced)
			result[ar.Name] = rt
		}
	}

	return result, nil
}

type cancellableSync struct {
	cancel context.CancelFunc
	sync   *data.GenericSync
}

type analyzer struct {
	C       chan analysisResult
	updates chan *ast.Compiler
	opa     *sdk.OPA
	prefix  ast.Ref
	entry   ast.Ref
	logger  logging.Logger
}

type analysisResult struct {
	Refs []ref
}

func newAnalyzer(ctx context.Context, bs []byte, replicatePath, analysisEntrypoint string, logger logging.Logger) (*analyzer, error) {

	a := &analyzer{
		C:       make(chan analysisResult),
		updates: make(chan *ast.Compiler, 1),
		logger:  logger,
	}

	var err error

	a.prefix, err = ast.PtrRef(ast.DefaultRootDocument, replicatePath)
	if err != nil {
		return nil, err
	}
	a.entry, err = ast.PtrRef(ast.DefaultRootDocument, analysisEntrypoint)
	if err != nil {
		return nil, err
	}

	go a.loop(ctx)

	store := inmem.New()

	err = storage.Txn(ctx, store, storage.TransactionParams{Write: true}, func(txn storage.Transaction) error {
		_, err := store.Register(ctx, txn, storage.TriggerConfig{OnCommit: a.trigger})
		return err
	})
	if err != nil {
		return nil, err
	}

	a.opa, err = sdk.New(ctx, sdk.Options{Config: bytes.NewBuffer(bs), Store: store, Logger: logger})
	if err != nil {
		return nil, err
	}

	return a, nil
}

type ref struct {
	Resource string
}

func (a *analyzer) Stop(ctx context.Context) error {
	close(a.C)
	a.opa.Stop(ctx)
	return nil
}

func (a *analyzer) trigger(_ context.Context, txn storage.Transaction, event storage.TriggerEvent) {
	compiler := plugins.GetCompilerOnContext(event.Context)
	a.logger.Debug("Analyzer received storage trigger callback (txn=%d, compiler=%p)", txn.ID(), compiler)
	if compiler == nil {
		return
	}
	a.updates <- compiler
}

func (a *analyzer) loop(ctx context.Context) {
	for {
		select {
		case compiler := <-a.updates:
			refs, missing, err := analyzeRefs(compiler, []ast.Ref{a.entry}, a.prefix, a.logger)
			if err != nil {
				a.logger.Error("Failed to analyze refs: %v", err)
				continue
			}
			if len(missing) > 0 {
				a.logger.Debug("Analysis could not find entrypoints %v, skipping update", missing)
				continue
			}
			a.C <- analysisResult{Refs: refs}
		case <-ctx.Done():
			logrus.Info("Analyzer shutting down")
			return
		}
	}
}

func analyzeRefs(c *ast.Compiler, entrypoints []ast.Ref, prefix ast.Ref, logger logging.Logger) ([]ref, []ast.Ref, error) {
	logger.Debug("Analyzing dependencies for references to %v starting from %v", prefix, entrypoints)
	resultMap := map[string]struct{}{}
	visited := map[*ast.Rule]struct{}{}
	var queue []*ast.Rule

	missing := []ast.Ref{}

	for _, ref := range entrypoints {
		rules := c.GetRulesForVirtualDocument(ref)
		if len(rules) == 0 {
			missing = append(missing, ref)
		}
		queue = append(queue, rules...)
	}
	if len(missing) > 0 {
		return nil, missing, nil
	}

	for len(queue) > 0 {
		var next *ast.Rule
		next, queue = queue[0], queue[1:]
		if _, ok := visited[next]; ok {
			continue
		}
		visited[next] = struct{}{}
		for a := range c.Graph.Dependencies(next) {
			queue = append(queue, a.(*ast.Rule))
		}
		deps, err := dependencies.Minimal(next)
		if err != nil {
			logrus.Errorf("Analysis error for %v: %v", next.Location, err)
			continue
		}
		logrus.Debugf("Analyzed %v and found %v", next.Location, deps)
		for _, ref := range deps {
			if ref.HasPrefix(prefix) && len(ref) > len(prefix) {
				if s, ok := ref[len(prefix)].Value.(ast.String); ok {
					resultMap[string(s)] = struct{}{}
				}
			}
		}
	}

	var result []ref
	for x := range resultMap {
		result = append(result, ref{Resource: x})
	}

	sort.Slice(result, func(i, j int) bool {
		return result[i].Resource < result[j].Resource
	})

	return result, nil, nil
}

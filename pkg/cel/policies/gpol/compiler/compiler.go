func (c *compilerImpl) createBaseGpolEnv(libsctx libs.Context, namespace string) (*environment.EnvSet, *compiler.VariablesProvider, error) {
	baseOpts := compiler.DynamicResourceEnvOptions()

	baseOpts = append(baseOpts,
		cel.Variable(compiler.NamespaceObjectKey, compiler.NamespaceType.CelType()),
		cel.Variable(compiler.ObjectKey, cel.DynType),
		cel.Variable(compiler.OldObjectKey, cel.DynType),
		cel.Variable(compiler.RequestKey, compiler.RequestType.CelType()),
		cel.Types(compiler.NamespaceType.CelType()),
		cel.Types(compiler.RequestType.CelType()),
		cel.Variable(compiler.VariablesKey, compiler.VariablesType),
	)

	base := environment.MustBaseEnvSet(gpolCompilerVersion)
	env, err := base.Env(environment.StoredExpressions)
	if err != nil {
		return nil, nil, err
	}

	variablesProvider := compiler.NewVariablesProvider(env.CELTypeProvider())
	declProvider := apiservercel.NewDeclTypeProvider(compiler.NamespaceType, compiler.RequestType)
	declOptions, err := declProvider.EnvOptions(variablesProvider)
	if err != nil {
		return nil, nil, err
	}

	baseOpts = append(baseOpts, declOptions...)

	libEnvOpts := []cel.EnvOption{
		ext.NativeTypes(reflect.TypeFor[libs.Exception](), ext.ParseStructTags(true)),
		cel.Variable(compiler.ExceptionsKey, types.NewObjectType("libs.Exception")),
		generator.Lib(
			generator.Context{ContextInterface: libsctx},
			namespace,
			generator.Latest(),
		),
		globalcontext.Lib(
			globalcontext.Context{ContextInterface: libsctx},
			globalcontext.Latest(),
		),
		resource.Lib(
			resource.Context{ContextInterface: libsctx},
			namespace,
			resource.Latest(),
		),
		image.Lib(
			image.Latest(),
		),
		imagedata.Lib(
			imagedata.Context{ContextInterface: libsctx},
			imagedata.Latest(),
		),
		hash.Lib(
			hash.Latest(),
		),
		math.Lib(
			math.Latest(),
		),
		json.Lib(
			&json.JsonImpl{},
			json.Latest(),
		),
		yaml.Lib(
			&yaml.YamlImpl{},
			yaml.Latest(),
		),
		random.Lib(
			random.Latest(),
		),
		x509.Lib(
			x509.Latest(),
		),
		time.Lib(
			time.Latest(),
		),
		transform.Lib(
			transform.Latest(),
		),
		gzip.Lib(
			gzip.Latest(),
		),
		http.Lib(
			http.Context{ContextInterface: libs.NewMockAwareHTTPContext(compiler.NewLazyCELHTTPContext(namespace), libsctx.GetHTTPMocks())},
			http.Latest(),
		),
	}

	extendedBase, err := base.Extend(
		environment.VersionedOptions{
			IntroducedVersion: gpolCompilerVersion,
			EnvOptions:        baseOpts,
		},
		environment.VersionedOptions{
			IntroducedVersion: gpolCompilerVersion,
			EnvOptions:        libEnvOpts,
		},
	)
	if err != nil {
		return nil, nil, err
	}
	return extendedBase, variablesProvider, nil
}
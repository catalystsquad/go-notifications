# [1.1.0](https://github.com/catalystsquad/go-notifications/compare/v1.0.4...v1.1.0) (2023-05-31)


### Features

* upgrade go-scheduler, implement db conn pooling config ([#8](https://github.com/catalystsquad/go-notifications/issues/8)) ([eed4033](https://github.com/catalystsquad/go-notifications/commit/eed4033fa79716658f8d1eda86dbcdd649ce7497))

## [1.0.4](https://github.com/catalystsquad/go-notifications/compare/v1.0.3...v1.0.4) (2022-11-30)


### Bug Fixes

* Fix client initialization. Global init doesn't work because we rely on cobra to read in env vars, so init has to happen after that or we don't get the env vars ([#5](https://github.com/catalystsquad/go-notifications/issues/5)) ([4d4ff80](https://github.com/catalystsquad/go-notifications/commit/4d4ff80470e8bfa912983d1313e3ab597defc3c3))

## [1.0.3](https://github.com/catalystsquad/go-notifications/compare/v1.0.2...v1.0.3) (2022-11-29)


### Bug Fixes

* Tag without v ([#4](https://github.com/catalystsquad/go-notifications/issues/4)) ([753d7bd](https://github.com/catalystsquad/go-notifications/commit/753d7bd5f74da378ee24d57cc8593182191cfee6))

## [1.0.2](https://github.com/catalystsquad/go-notifications/compare/v1.0.1...v1.0.2) (2022-11-29)


### Bug Fixes

* Fix image push ([#3](https://github.com/catalystsquad/go-notifications/issues/3)) ([898f7b4](https://github.com/catalystsquad/go-notifications/commit/898f7b4fb448400e4551fd43e642bbca8eab1e35))

## [1.0.1](https://github.com/catalystsquad/go-notifications/compare/v1.0.0...v1.0.1) (2022-11-29)


### Bug Fixes

* Fix image build ([#2](https://github.com/catalystsquad/go-notifications/issues/2)) ([a871e37](https://github.com/catalystsquad/go-notifications/commit/a871e3702a9b9ce4d22aa96a5d0d72d84b86327a))

# 1.0.0 (2022-11-28)


### Bug Fixes

* Initial release ([#1](https://github.com/catalystsquad/go-notifications/issues/1)) ([d4b035f](https://github.com/catalystsquad/go-notifications/commit/d4b035f78b49d48740e1fd6648bee64f7614b6ff))

# 1.0.0 (2022-06-07)


### Bug Fixes

* Initial commit. Added example cobra cli with viper config, config val… ([#1](https://github.com/catalystsquad/template-go-cobra-app/issues/1)) ([b71e02f](https://github.com/catalystsquad/template-go-cobra-app/commit/b71e02f901152916e4c7c08e21461338ad3d04d8))

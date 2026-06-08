package options

import "errors"

func (o *Options) Validate() error {
	var errs error
	errs = errors.Join(errs, o.Model.Validate())
	errs = errors.Join(errs, o.Security.Validate())
	return errs
}

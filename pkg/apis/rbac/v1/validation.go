package rbacv1

import (
	"fmt"

	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/reflect/protoregistry"
)

func (c *Config) Validate() error {
	if err := c.validateRoles(); err != nil {
		return err
	}
	if err := c.validateRoleBindings(); err != nil {
		return err
	}
	return nil
}

func (c *Config) validateRoles() error {
	uniqueRoleIds := make(map[string]struct{})
	for _, r := range c.GetRoles() {
		roleId := r.GetId()
		if roleId == "" {
			return fmt.Errorf("role id cannot be empty")
		}
		if _, ok := uniqueRoleIds[roleId]; ok {
			return fmt.Errorf("duplicate role id %q", roleId)
		}
		uniqueRoleIds[roleId] = struct{}{}
		svcName := r.GetService()

		// ensure service and method names exist
		var svcDesc protoreflect.ServiceDescriptor
		if maybeSvcDesc, err := protoregistry.GlobalFiles.FindDescriptorByName(protoreflect.FullName(r.GetService())); err == nil {
			var ok bool
			if svcDesc, ok = maybeSvcDesc.(protoreflect.ServiceDescriptor); !ok {
				return fmt.Errorf("invalid role %q: %q is not a service", roleId, svcName)
			}
		} else {
			return fmt.Errorf("invalid role %q: service %q not found", roleId, svcName)
		}
		uniqueMethodNames := make(map[string]struct{})
		for _, m := range r.GetAllowedMethods() {
			methodName := m.GetName()
			if methodName == "" {
				return fmt.Errorf("invalid role %q: method name cannot be empty", roleId)
			}
			if _, ok := uniqueMethodNames[methodName]; ok {
				return fmt.Errorf("invalid role %q: duplicate method name %q", roleId, m)
			}
			uniqueMethodNames[methodName] = struct{}{}
			mtdDesc := svcDesc.Methods().ByName(protoreflect.Name(methodName))
			if mtdDesc == nil {
				return fmt.Errorf("invalid role %q: service %q does not contain method %q", roleId, svcName, methodName)
			}
			scopeConfigured := m.Scope != nil
			if m.Scope != nil {
				// check that the scope value is valid
				if m.Scope.Type().Descriptor().Values().ByNumber(m.Scope.Number()) == nil {
					return fmt.Errorf("invalid role %q: method %q has invalid scope value %q", roleId, methodName, m.Scope.String())
				}
			}
			// scope must be configured iff the method has scope semantics enabled
			var scopeEnabled bool
			if opts := proto.GetExtension(mtdDesc.Options(), E_Scope).(*ScopeOptions); opts != nil {
				scopeEnabled = opts.GetEnabled()
			}
			switch {
			case scopeConfigured && !scopeEnabled:
				return fmt.Errorf("invalid role %q: method %q does not support scopes", roleId, methodName)
			case !scopeConfigured && scopeEnabled:
				return fmt.Errorf("invalid role %q: method %q requires a scope", roleId, methodName)
			}
		}
	}

	return nil
}

func (c *Config) validateRoleBindings() error {
	uniqueRoleBindingIds := make(map[string]struct{})
	for _, rb := range c.GetRoleBindings() {
		rbId := rb.GetId()
		if rbId == "" {
			return fmt.Errorf("role id cannot be empty")
		}
		if _, ok := uniqueRoleBindingIds[rbId]; ok {
			return fmt.Errorf("duplicate role id %q", rbId)
		}
		uniqueRoleBindingIds[rbId] = struct{}{}

		roleId := rb.GetRoleId()
		if roleId == "" {
			return fmt.Errorf("invalid role binding %q: role id cannot be empty", rbId)
		}

		// ensure the role with the given id exists
		var found bool
		for _, role := range c.GetRoles() {
			if role.GetId() == roleId {
				found = true
				break
			}
		}
		if !found {
			return fmt.Errorf("invalid role binding %q: role %q not found", rbId, roleId)
		}

		// ensure at least one user is configured
		if len(rb.GetUsers()) == 0 {
			return fmt.Errorf("invalid role binding %q: at least one user must be configured", rbId)
		}
	}
	return nil
}

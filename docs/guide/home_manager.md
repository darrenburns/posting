## Overview

Posting can be configured with Nix using Home Manager. You can configure all settings and add custom themes using the `programs.posting` option.

## Settings

The option `programs.posting.settings` is an attribute set containing all of the settings for Posting. See [configuration](./configuration.md) for documentation on settings. There is data validation for settings with a limited set of acceptible values, such as `settings.animation`. `settings.theme` will only accept one of the built-in themes or one of the themes defined in `posting.themes`. In addition, it will accept `xresources-dark` and `xresources-light` if `settings.use_xresources` is set to `true`.

## Themes

The option `programs.posting.themes` is an attribute set where each attribute's name is the name of a theme and its value is the theme's definition. These themes will be written to the directory specified by `settings.theme_directory`. There is data validation for the basic color values (primary, secondary, etc.), but _not_ for the syntax highlighting values. See [themes](./themes.md) for documentation on theming. **Note: Since the name of the theme is determined by its attribute name, the `name` option of the theme definition is not set in the attribute's value.**

## Immutability

Configuring Posting with Nix will make the configuration and theme files immutable. This will not prevent you from adjusting settings during runtime, but it will prevent those adjustments from being persisted.

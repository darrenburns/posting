from yaml import load, dump
import yaml

try:
    from yaml import CLoader as Loader, Dumper as Dumper
except ImportError:
    from yaml import Loader, Dumper


def str_presenter(dumper: Dumper, data: str) -> yaml.ScalarNode:
    if data.count("\n") > 0:
        data = "\n".join(
            [line.rstrip() for line in data.splitlines()]
        )  # Remove any trailing spaces, then put it back together again
        return dumper.represent_scalar("tag:yaml.org,2002:str", data, style="|")
    return dumper.represent_scalar("tag:yaml.org,2002:str", data)


yaml.add_representer(str, str_presenter)
yaml.representer.SafeRepresenter.add_representer(str, str_presenter)


__all__ = ["load", "dump", "Loader", "Dumper"]

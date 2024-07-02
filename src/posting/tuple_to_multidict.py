from __future__ import annotations
from collections import defaultdict
from typing import TypeVar

K = TypeVar("K")
V = TypeVar("V")


def tuples_to_dict(tuple_list: list[tuple[K, V]]) -> dict[K, list[V]]:
    result: dict[K, list[V]] = defaultdict(list)
    for key, value in tuple_list:
        result[key].append(value)
    return result

# Repository Coverage

[Full report](https://htmlpreview.github.io/?https://github.com/darrenburns/posting/blob/python-coverage-comment-action-data/htmlcov/index.html)

| Name                                                  |    Stmts |     Miss |   Cover |   Missing |
|------------------------------------------------------ | -------: | -------: | ------: | --------: |
| src/posting/\_\_init\_\_.py                           |        0 |        0 |    100% |           |
| src/posting/\_\_main\_\_.py                           |       73 |       44 |     40% |19-26, 30-39, 61-69, 77-89, 103-122 |
| src/posting/app.py                                    |      433 |       61 |     86% |160, 164, 189, 191, 193, 199, 231-243, 263, 275, 316-317, 327, 332-336, 352-368, 440-442, 498-503, 606, 656, 670-672, 676, 679-680, 705-706, 716, 736-737, 748-755, 764-765 |
| src/posting/collection.py                             |      251 |       74 |     71% |22-25, 92-100, 162-170, 178-181, 184-199, 275-276, 282-315, 330-331, 360-361, 383-390 |
| src/posting/commands.py                               |       45 |        4 |     91% |7, 27, 62, 64 |
| src/posting/config.py                                 |       93 |        4 |     96% |166, 186-187, 197 |
| src/posting/help\_screen.py                           |       62 |        4 |     94% |   155-162 |
| src/posting/highlight\_url.py                         |        0 |        0 |    100% |           |
| src/posting/highlighters.py                           |       46 |        2 |     96% |    37, 61 |
| src/posting/importing/open\_api.py                    |      130 |      115 |     12% |36-38, 43-58, 62-87, 97-142, 148-163, 167-255, 259-266 |
| src/posting/jump\_overlay.py                          |       51 |        3 |     94% |11, 63, 70 |
| src/posting/jumper.py                                 |       33 |        1 |     97% |        54 |
| src/posting/locations.py                              |       18 |        3 |     83% |26, 31, 35 |
| src/posting/messages.py                               |        6 |        0 |    100% |           |
| src/posting/request\_headers.py                       |        8 |        0 |    100% |           |
| src/posting/save\_request.py                          |        9 |        0 |    100% |           |
| src/posting/suggesters.py                             |        1 |        1 |      0% |         2 |
| src/posting/themes.py                                 |       38 |        2 |     95% |     45-46 |
| src/posting/tuple\_to\_multidict.py                   |       10 |        4 |     60% |     10-13 |
| src/posting/types.py                                  |        3 |        0 |    100% |           |
| src/posting/user\_host.py                             |       10 |        2 |     80% |     12-13 |
| src/posting/variables.py                              |       93 |       22 |     76% |35, 43-44, 68, 77, 87, 97, 102, 111, 117, 121, 127, 132-134, 143-145, 149-150, 183, 186 |
| src/posting/version.py                                |        2 |        0 |    100% |           |
| src/posting/widgets/\_\_init\_\_.py                   |        0 |        0 |    100% |           |
| src/posting/widgets/center\_middle.py                 |        3 |        0 |    100% |           |
| src/posting/widgets/collection/browser.py             |      219 |       24 |     89% |77, 86, 118, 193, 201, 215-216, 225, 233, 255, 269-271, 337-338, 416, 472-480 |
| src/posting/widgets/collection/new\_request\_modal.py |       72 |        4 |     94% |129, 147, 161, 165 |
| src/posting/widgets/datatable.py                      |       91 |       31 |     66% |67, 70-73, 97-112, 115-127, 135-140 |
| src/posting/widgets/input.py                          |        5 |        0 |    100% |           |
| src/posting/widgets/key\_value.py                     |       95 |       16 |     83% |53, 104-107, 110, 116-124, 176-178, 185 |
| src/posting/widgets/request/\_\_init\_\_.py           |        0 |        0 |    100% |           |
| src/posting/widgets/request/form\_editor.py           |       29 |        6 |     79% | 24-29, 46 |
| src/posting/widgets/request/header\_editor.py         |       50 |        7 |     86% |99-100, 103-107 |
| src/posting/widgets/request/method\_selection.py      |       29 |        1 |     97% |        80 |
| src/posting/widgets/request/query\_editor.py          |       29 |        4 |     86% |43-44, 50-51 |
| src/posting/widgets/request/request\_auth.py          |      105 |       36 |     66% |22, 50, 122-128, 135-152, 162-165, 170-182, 193-196 |
| src/posting/widgets/request/request\_body.py          |       81 |       66 |     19% |38-106, 109-114, 119-130 |
| src/posting/widgets/request/request\_editor.py        |       73 |        5 |     93% |   126-131 |
| src/posting/widgets/request/request\_metadata.py      |       40 |        3 |     92% |     38-40 |
| src/posting/widgets/request/request\_options.py       |       79 |       16 |     80% |123-128, 133, 140-141, 146-153 |
| src/posting/widgets/request/url\_bar.py               |      134 |       10 |     93% |74, 82, 221-224, 236, 244, 247-248, 264-265 |
| src/posting/widgets/response/cookies\_table.py        |        8 |        0 |    100% |           |
| src/posting/widgets/response/response\_area.py        |       96 |       19 |     80% |84-85, 105-108, 116, 132, 136, 144-152, 156-159 |
| src/posting/widgets/response/response\_body.py        |       11 |        0 |    100% |           |
| src/posting/widgets/response/response\_headers.py     |        9 |        0 |    100% |           |
| src/posting/widgets/response/response\_trace.py       |       38 |        5 |     87% | 63, 75-78 |
| src/posting/widgets/select.py                         |       18 |        7 |     61% |17-20, 23-26, 30 |
| src/posting/widgets/tabbed\_content.py                |       12 |        6 |     50% |14-16, 19-21 |
| src/posting/widgets/text\_area.py                     |      254 |      103 |     59% |122, 148-149, 152-154, 157-159, 191-193, 197, 200, 231-240, 243-266, 269-299, 399, 404, 407-412, 415, 418, 421, 424, 427, 430, 433, 436, 439-451, 454, 457, 461-475, 479-486, 490-497, 533, 537, 545, 549 |
| src/posting/widgets/tree.py                           |       22 |        6 |     73% |     34-39 |
| src/posting/widgets/variable\_autocomplete.py         |       38 |        6 |     84% |     71-78 |
| src/posting/widgets/variable\_input.py                |       10 |        0 |    100% |           |
| src/posting/xresources.py                             |       22 |       17 |     23% |     20-43 |
|                                             **TOTAL** | **3087** |  **744** | **76%** |           |


## Setup coverage badge

Below are examples of the badges you can use in your main branch `README` file.

### Direct image

[![Coverage badge](https://raw.githubusercontent.com/darrenburns/posting/python-coverage-comment-action-data/badge.svg)](https://htmlpreview.github.io/?https://github.com/darrenburns/posting/blob/python-coverage-comment-action-data/htmlcov/index.html)

This is the one to use if your repository is private or if you don't want to customize anything.

### [Shields.io](https://shields.io) Json Endpoint

[![Coverage badge](https://img.shields.io/endpoint?url=https://raw.githubusercontent.com/darrenburns/posting/python-coverage-comment-action-data/endpoint.json)](https://htmlpreview.github.io/?https://github.com/darrenburns/posting/blob/python-coverage-comment-action-data/htmlcov/index.html)

Using this one will allow you to [customize](https://shields.io/endpoint) the look of your badge.
It won't work with private repositories. It won't be refreshed more than once per five minutes.

### [Shields.io](https://shields.io) Dynamic Badge

[![Coverage badge](https://img.shields.io/badge/dynamic/json?color=brightgreen&label=coverage&query=%24.message&url=https%3A%2F%2Fraw.githubusercontent.com%2Fdarrenburns%2Fposting%2Fpython-coverage-comment-action-data%2Fendpoint.json)](https://htmlpreview.github.io/?https://github.com/darrenburns/posting/blob/python-coverage-comment-action-data/htmlcov/index.html)

This one will always be the same color. It won't work for private repos. I'm not even sure why we included it.

## What is that?

This branch is part of the
[python-coverage-comment-action](https://github.com/marketplace/actions/python-coverage-comment)
GitHub Action. All the files in this branch are automatically generated and may be
overwritten at any moment.
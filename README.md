# Repository Coverage

[Full report](https://htmlpreview.github.io/?https://github.com/darrenburns/posting/blob/python-coverage-comment-action-data/htmlcov/index.html)

| Name                                                  |    Stmts |     Miss |   Cover |   Missing |
|------------------------------------------------------ | -------: | -------: | ------: | --------: |
| src/posting/\_\_init\_\_.py                           |        3 |        0 |    100% |           |
| src/posting/\_\_main\_\_.py                           |       78 |       45 |     42% |21-28, 32-41, 63-71, 79-91, 105-124, 143 |
| src/posting/app.py                                    |      573 |      116 |     80% |191, 195, 230-231, 239-246, 267, 273-288, 300, 302, 304, 320-321, 331-333, 340, 361-362, 399-400, 407-419, 439, 451, 518, 526, 543-559, 681-693, 885-901, 910-933, 938-949, 959-960, 963, 1001, 1004, 1007, 1018, 1023, 1033, 1044-1052, 1061-1062, 1076-1077, 1088-1095, 1104-1105, 1128-1129 |
| src/posting/collection.py                             |      290 |       72 |     75% |23-26, 42-48, 56, 120, 211-212, 226-229, 232-243, 280-281, 328-329, 335-368, 383-384, 413-414, 433-440 |
| src/posting/commands.py                               |       47 |        5 |     89% |7, 25, 65, 67, 93 |
| src/posting/config.py                                 |      112 |        2 |     98% |  222, 239 |
| src/posting/exit\_codes.py                            |        1 |        0 |    100% |           |
| src/posting/files.py                                  |       63 |       27 |     57% |68, 92-94, 110-146 |
| src/posting/help\_screen.py                           |       62 |        4 |     94% |   155-162 |
| src/posting/highlight\_url.py                         |        0 |        0 |    100% |           |
| src/posting/highlighters.py                           |       51 |        2 |     96% |    51, 64 |
| src/posting/importing/curl.py                         |      136 |       57 |     58% |113, 136, 150, 157, 166-227, 232-269 |
| src/posting/importing/open\_api.py                    |      130 |      115 |     12% |36-38, 43-58, 62-87, 97-142, 148-163, 167-255, 259-266 |
| src/posting/jump\_overlay.py                          |       49 |        3 |     94% |11, 60, 66 |
| src/posting/jumper.py                                 |       34 |        1 |     97% |        55 |
| src/posting/locations.py                              |       18 |        3 |     83% |26, 31, 35 |
| src/posting/messages.py                               |        6 |        0 |    100% |           |
| src/posting/request\_headers.py                       |        8 |        0 |    100% |           |
| src/posting/save\_request.py                          |        9 |        0 |    100% |           |
| src/posting/scripts.py                                |       76 |       16 |     79% |16, 63-65, 69-70, 103-104, 131, 136, 162, 168, 197-200 |
| src/posting/suggesters.py                             |        1 |        1 |      0% |         2 |
| src/posting/themes.py                                 |      162 |       10 |     94% |145, 240, 325-326, 333-338 |
| src/posting/tuple\_to\_multidict.py                   |       10 |        0 |    100% |           |
| src/posting/types.py                                  |        3 |        0 |    100% |           |
| src/posting/user\_host.py                             |       11 |        2 |     82% |     14-15 |
| src/posting/variables.py                              |      102 |       21 |     79% |66-67, 103, 112, 122, 132, 137, 146, 152, 156, 162, 167-169, 178-180, 184-185, 219, 222 |
| src/posting/version.py                                |        2 |        0 |    100% |           |
| src/posting/widgets/\_\_init\_\_.py                   |        0 |        0 |    100% |           |
| src/posting/widgets/center\_middle.py                 |        3 |        0 |    100% |           |
| src/posting/widgets/collection/browser.py             |      315 |       79 |     75% |120, 130, 139, 171, 266, 274, 288-289, 298, 306, 328, 342-344, 361, 365, 370, 434-440, 446-454, 457-482, 489-496, 499-525, 538-539, 669-677 |
| src/posting/widgets/collection/new\_request\_modal.py |      122 |       12 |     90% |18, 48, 158, 178, 192-196, 199-203, 212, 217, 226-230 |
| src/posting/widgets/confirmation.py                   |       39 |       21 |     46% |51-57, 60-64, 67-72, 76, 80, 84 |
| src/posting/widgets/datatable.py                      |       91 |       31 |     66% |67, 70-73, 97-112, 115-127, 135-140 |
| src/posting/widgets/input.py                          |       61 |        2 |     97% |    33, 72 |
| src/posting/widgets/key\_value.py                     |       93 |       16 |     83% |22, 73-76, 79, 85-93, 145-147, 154 |
| src/posting/widgets/request/\_\_init\_\_.py           |        0 |        0 |    100% |           |
| src/posting/widgets/request/form\_editor.py           |       29 |        0 |    100% |           |
| src/posting/widgets/request/header\_editor.py         |       52 |        5 |     90% |     95-99 |
| src/posting/widgets/request/method\_selection.py      |       29 |        1 |     97% |        75 |
| src/posting/widgets/request/query\_editor.py          |       31 |        4 |     87% |29-30, 36-37 |
| src/posting/widgets/request/request\_auth.py          |      104 |       39 |     62% |21, 49, 117-127, 134-151, 161-164, 169-181, 192-195 |
| src/posting/widgets/request/request\_body.py          |       82 |       61 |     26% |43-47, 52-57, 59-106, 111-116, 121-132 |
| src/posting/widgets/request/request\_editor.py        |       81 |        3 |     96% |23, 125, 138 |
| src/posting/widgets/request/request\_metadata.py      |       40 |        3 |     92% |     17-19 |
| src/posting/widgets/request/request\_options.py       |       79 |       16 |     80% |116-121, 126, 133-134, 139-146 |
| src/posting/widgets/request/request\_scripts.py       |       81 |       32 |     60% |53-71, 81-110, 118, 126, 226-229 |
| src/posting/widgets/request/url\_bar.py               |      166 |       18 |     89% |33-34, 70, 78, 114-117, 150-151, 183-184, 190-191, 207-210, 252-253 |
| src/posting/widgets/response/cookies\_table.py        |        8 |        0 |    100% |           |
| src/posting/widgets/response/response\_area.py        |      110 |       21 |     81% |84-85, 110-111, 132-135, 141, 161, 165, 173-181, 185-188 |
| src/posting/widgets/response/response\_body.py        |       12 |        0 |    100% |           |
| src/posting/widgets/response/response\_headers.py     |        9 |        0 |    100% |           |
| src/posting/widgets/response/response\_trace.py       |       38 |        5 |     87% | 63, 75-78 |
| src/posting/widgets/response/script\_output.py        |       61 |        1 |     98% |        78 |
| src/posting/widgets/rich\_log.py                      |       26 |        0 |    100% |           |
| src/posting/widgets/select.py                         |       18 |        5 |     72% |17-20, 24, 30 |
| src/posting/widgets/tabbed\_content.py                |       12 |        6 |     50% |14-16, 19-21 |
| src/posting/widgets/text\_area.py                     |      278 |      104 |     63% |80-82, 128, 158-159, 179-188, 191-214, 217-248, 350, 355, 358-363, 366, 369, 372, 375, 378, 381, 384, 387, 390-413, 416, 419, 423-437, 441-448, 452-459, 463, 503, 515, 520-525 |
| src/posting/widgets/tree.py                           |       22 |        6 |     73% |     34-39 |
| src/posting/widgets/variable\_autocomplete.py         |       40 |        7 |     82% | 49, 74-80 |
| src/posting/widgets/variable\_input.py                |       20 |        0 |    100% |           |
| src/posting/xresources.py                             |       24 |       17 |     29% |     22-45 |
|                                             **TOTAL** | **4213** | **1017** | **76%** |           |


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
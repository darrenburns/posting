# Repository Coverage

[Full report](https://htmlpreview.github.io/?https://github.com/darrenburns/posting/blob/python-coverage-comment-action-data/htmlcov/index.html)

| Name                                                  |    Stmts |     Miss |   Cover |   Missing |
|------------------------------------------------------ | -------: | -------: | ------: | --------: |
| src/posting/\_\_init\_\_.py                           |        3 |        0 |    100% |           |
| src/posting/\_\_main\_\_.py                           |       77 |       45 |     42% |20-27, 31-40, 62-70, 78-90, 104-123, 142 |
| src/posting/app.py                                    |      598 |      135 |     77% |197, 201, 236-237, 245-252, 273, 279-294, 306, 308, 310, 326-327, 337-339, 346, 367-368, 405-406, 413-425, 445, 457, 524, 532, 549-565, 687-699, 894-910, 919-942, 947-965, 975-976, 979, 987, 1005-1009, 1039, 1042, 1045, 1056, 1059-1089, 1094, 1104, 1115-1123, 1132-1133, 1147-1148, 1159-1166, 1175-1176, 1199-1200 |
| src/posting/collection.py                             |      332 |       72 |     78% |23-26, 42-48, 120, 211-212, 226-229, 232-243, 280-281, 296, 408-409, 415-448, 463-464, 493-494, 513-520 |
| src/posting/commands.py                               |       55 |        6 |     89% |7, 28, 47, 87, 89, 118 |
| src/posting/config.py                                 |      114 |        2 |     98% |  225, 242 |
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
| src/posting/scripts.py                                |       78 |       17 |     78% |16, 57, 77-79, 83-84, 117-118, 145, 150, 176, 182, 211-214 |
| src/posting/suggesters.py                             |        1 |        1 |      0% |         2 |
| src/posting/themes.py                                 |      176 |        7 |     96% |236, 335-336, 345-346, 350-351 |
| src/posting/tuple\_to\_multidict.py                   |       10 |        0 |    100% |           |
| src/posting/types.py                                  |        3 |        0 |    100% |           |
| src/posting/user\_host.py                             |       11 |        2 |     82% |     14-15 |
| src/posting/variables.py                              |       66 |        9 |     86% |66-67, 119, 128, 150-154 |
| src/posting/version.py                                |        2 |        0 |    100% |           |
| src/posting/widgets/\_\_init\_\_.py                   |        0 |        0 |    100% |           |
| src/posting/widgets/center\_middle.py                 |        3 |        0 |    100% |           |
| src/posting/widgets/collection/browser.py             |      315 |       79 |     75% |120, 130, 139, 171, 266, 274, 288-289, 298, 306, 328, 342-344, 361, 365, 370, 434-440, 446-454, 457-482, 489-496, 499-525, 538-539, 669-677 |
| src/posting/widgets/collection/new\_request\_modal.py |      122 |       12 |     90% |18, 48, 158, 178, 192-196, 199-203, 212, 217, 226-230 |
| src/posting/widgets/confirmation.py                   |       39 |       21 |     46% |51-57, 60-64, 67-72, 76, 80, 84 |
| src/posting/widgets/datatable.py                      |       95 |       32 |     66% |67, 72, 75-78, 102-117, 120-132, 140-145 |
| src/posting/widgets/input.py                          |       61 |       36 |     41% |24-36, 40, 56-57, 62-96 |
| src/posting/widgets/key\_value.py                     |       93 |       16 |     83% |22, 73-76, 79, 85-93, 145-147, 154 |
| src/posting/widgets/request/\_\_init\_\_.py           |        0 |        0 |    100% |           |
| src/posting/widgets/request/form\_editor.py           |       29 |        0 |    100% |           |
| src/posting/widgets/request/header\_editor.py         |       52 |        5 |     90% |     95-99 |
| src/posting/widgets/request/method\_selection.py      |       29 |        2 |     93% |    75, 86 |
| src/posting/widgets/request/query\_editor.py          |       31 |        4 |     87% |29-30, 36-37 |
| src/posting/widgets/request/request\_auth.py          |      104 |       39 |     62% |21, 49, 117-127, 134-151, 161-164, 169-181, 192-195 |
| src/posting/widgets/request/request\_body.py          |       11 |        0 |    100% |           |
| src/posting/widgets/request/request\_editor.py        |       81 |        3 |     96% |23, 125, 138 |
| src/posting/widgets/request/request\_metadata.py      |       40 |        3 |     92% |     17-19 |
| src/posting/widgets/request/request\_options.py       |       79 |       16 |     80% |116-121, 126, 133-134, 139-146 |
| src/posting/widgets/request/request\_scripts.py       |       81 |       32 |     60% |53-71, 81-110, 118, 126, 226-229 |
| src/posting/widgets/request/url\_bar.py               |      158 |       35 |     78% |33-34, 70, 81, 103-106, 139-140, 172-173, 179-180, 184, 187-211, 241-242 |
| src/posting/widgets/response/cookies\_table.py        |        8 |        0 |    100% |           |
| src/posting/widgets/response/response\_area.py        |      110 |       21 |     81% |84-85, 110-111, 132-135, 141, 161, 165, 173-181, 185-188 |
| src/posting/widgets/response/response\_body.py        |       12 |        0 |    100% |           |
| src/posting/widgets/response/response\_headers.py     |        9 |        0 |    100% |           |
| src/posting/widgets/response/response\_trace.py       |       38 |        5 |     87% | 63, 75-78 |
| src/posting/widgets/response/script\_output.py        |       61 |        1 |     98% |        78 |
| src/posting/widgets/rich\_log.py                      |       26 |        0 |    100% |           |
| src/posting/widgets/select.py                         |       18 |        5 |     72% |17-20, 24, 30 |
| src/posting/widgets/tabbed\_content.py                |       12 |        6 |     50% |14-16, 19-21 |
| src/posting/widgets/text\_area.py                     |      371 |      183 |     51% |86-88, 134, 172-173, 193-202, 205-228, 231-262, 268, 271-275, 280-285, 287-334, 339-344, 349-360, 462, 467, 470-475, 478, 481, 484, 487, 490, 493, 496, 499, 502-525, 528, 531, 535-549, 553-560, 564-571, 575, 578-583, 588-599, 639, 651, 656-661 |
| src/posting/widgets/tree.py                           |       22 |        6 |     73% |     34-39 |
| src/posting/widgets/variable\_autocomplete.py         |       40 |        7 |     82% | 49, 74-80 |
| src/posting/widgets/variable\_input.py                |       20 |        0 |    100% |           |
| src/posting/xresources.py                             |       24 |       17 |     29% |     22-45 |
|                                             **TOTAL** | **4287** | **1094** | **74%** |           |


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
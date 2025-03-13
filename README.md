# Repository Coverage

[Full report](https://htmlpreview.github.io/?https://github.com/darrenburns/posting/blob/python-coverage-comment-action-data/htmlcov/index.html)

| Name                                                  |    Stmts |     Miss |   Cover |   Missing |
|------------------------------------------------------ | -------: | -------: | ------: | --------: |
| src/posting/\_\_init\_\_.py                           |        6 |        0 |    100% |           |
| src/posting/\_\_main\_\_.py                           |       77 |       46 |     40% |20-27, 31-40, 62-70, 78-90, 104-128, 147 |
| src/posting/\_start\_time.py                          |        2 |        0 |    100% |           |
| src/posting/app.py                                    |      627 |      148 |     76% |202, 206, 244-245, 253-260, 285, 291-306, 318, 320, 322, 339-340, 350-352, 359, 382-383, 412-413, 420-445, 465, 477, 544, 552, 569-585, 707-719, 727-737, 972-990, 1000-1025, 1031-1051, 1061-1062, 1065, 1073, 1091-1095, 1125, 1128, 1131, 1142, 1161-1162, 1187-1199, 1210, 1220, 1231-1239, 1248-1249, 1263-1264, 1275-1282, 1291-1292, 1317-1318 |
| src/posting/auth.py                                   |        8 |        3 |     62% |  8, 11-12 |
| src/posting/collection.py                             |      342 |       78 |     77% |29-38, 52, 118, 209-210, 224-227, 230-242, 281-285, 302, 414-415, 421-454, 469-470, 499-500, 519-526 |
| src/posting/commands.py                               |       56 |        5 |     91% |7, 28, 95, 97, 126 |
| src/posting/config.py                                 |      114 |        2 |     98% |  224, 241 |
| src/posting/exit\_codes.py                            |        1 |        0 |    100% |           |
| src/posting/files.py                                  |       63 |       26 |     59% |68, 94, 110-146 |
| src/posting/help\_data.py                             |        7 |        0 |    100% |           |
| src/posting/help\_screen.py                           |       56 |        4 |     93% |   145-152 |
| src/posting/highlight\_url.py                         |        0 |        0 |    100% |           |
| src/posting/highlighters.py                           |       51 |        2 |     96% |    51, 64 |
| src/posting/importing/curl.py                         |      142 |       62 |     56% |116, 139, 153, 160, 169-239, 244-281 |
| src/posting/importing/open\_api.py                    |      157 |       43 |     73% |40-42, 56, 86, 103-104, 113, 125-126, 145, 148-155, 158-162, 165-168, 250, 290-304, 313-320 |
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
| src/posting/variables.py                              |       66 |        5 |     92% |66-67, 119, 151, 154 |
| src/posting/version.py                                |        2 |        0 |    100% |           |
| src/posting/widgets/\_\_init\_\_.py                   |        0 |        0 |    100% |           |
| src/posting/widgets/center\_middle.py                 |        3 |        0 |    100% |           |
| src/posting/widgets/collection/browser.py             |      315 |       79 |     75% |120, 130, 139, 171, 266, 274, 288-289, 298, 306, 328, 342-344, 361, 365, 370, 434-440, 446-454, 457-482, 489-496, 499-525, 538-539, 669-677 |
| src/posting/widgets/collection/new\_request\_modal.py |      149 |        4 |     97% |18, 191, 271, 276 |
| src/posting/widgets/confirmation.py                   |       39 |       21 |     46% |51-57, 60-64, 67-72, 76, 80, 84 |
| src/posting/widgets/datatable.py                      |      166 |       37 |     78% |65, 96, 99, 103, 106-109, 130, 148, 154, 158, 161-173, 181-186, 193-194, 199-200, 212-213, 240-242 |
| src/posting/widgets/input.py                          |       17 |        1 |     94% |        20 |
| src/posting/widgets/key\_value.py                     |       93 |       16 |     83% |22, 73-76, 79, 85-93, 145-147, 154 |
| src/posting/widgets/request/\_\_init\_\_.py           |        0 |        0 |    100% |           |
| src/posting/widgets/request/form\_editor.py           |       30 |        0 |    100% |           |
| src/posting/widgets/request/header\_editor.py         |       54 |        6 |     89% |    98-103 |
| src/posting/widgets/request/method\_selection.py      |       29 |        2 |     93% |    75, 86 |
| src/posting/widgets/request/query\_editor.py          |       32 |        4 |     88% |31-32, 37-38 |
| src/posting/widgets/request/request\_auth.py          |      143 |       57 |     60% |24, 52, 96-99, 102, 105, 178-190, 197-220, 230-233, 238-260, 271-274 |
| src/posting/widgets/request/request\_body.py          |       23 |        0 |    100% |           |
| src/posting/widgets/request/request\_editor.py        |       76 |        3 |     96% |24, 97, 110 |
| src/posting/widgets/request/request\_metadata.py      |       40 |        3 |     92% |     17-19 |
| src/posting/widgets/request/request\_options.py       |       79 |       16 |     80% |116-121, 126, 133-134, 139-146 |
| src/posting/widgets/request/request\_scripts.py       |       81 |       32 |     60% |53-71, 81-110, 118, 126, 226-229 |
| src/posting/widgets/request/url\_bar.py               |      159 |       17 |     89% |34-35, 71, 104-107, 140-141, 173-174, 180-181, 196-199, 241-242 |
| src/posting/widgets/response/cookies\_table.py        |        8 |        0 |    100% |           |
| src/posting/widgets/response/response\_area.py        |      111 |       21 |     81% |85-86, 111-112, 133-136, 142, 162, 166, 174-182, 186-189 |
| src/posting/widgets/response/response\_body.py        |       12 |        0 |    100% |           |
| src/posting/widgets/response/response\_headers.py     |        9 |        0 |    100% |           |
| src/posting/widgets/response/response\_trace.py       |       38 |        5 |     87% | 63, 75-78 |
| src/posting/widgets/response/script\_output.py        |       61 |        1 |     98% |        78 |
| src/posting/widgets/rich\_log.py                      |       26 |        0 |    100% |           |
| src/posting/widgets/select.py                         |       18 |        5 |     72% |17-20, 24, 30 |
| src/posting/widgets/tabbed\_content.py                |       12 |        6 |     50% |14-16, 19-21 |
| src/posting/widgets/text\_area.py                     |      371 |      183 |     51% |86-88, 134, 172-173, 193-202, 205-228, 231-262, 268, 271-275, 280-285, 287-334, 339-344, 349-360, 462, 467, 470-475, 478, 481, 484, 487, 490, 493, 496, 499, 502-525, 528, 531, 535-549, 553-560, 564-571, 575, 578-583, 588-599, 639, 651, 656-661 |
| src/posting/widgets/tree.py                           |       26 |        8 |     69% |35-40, 54-55 |
| src/posting/widgets/variable\_autocomplete.py         |       40 |        7 |     82% | 49, 74-80 |
| src/posting/widgets/variable\_input.py                |       20 |        0 |    100% |           |
| src/posting/xresources.py                             |       24 |       17 |     29% |     22-45 |
| src/posting/yaml.py                                   |       14 |        4 |     71% |6-7, 12-15 |
|                                             **TOTAL** | **4498** | **1013** | **77%** |           |


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
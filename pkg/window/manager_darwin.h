#ifndef MANAGER_DARWIN_H
#define MANAGER_DARWIN_H

#ifdef __cplusplus
extern "C" {
#endif

void DarwinInitStatusItem(void);
void DarwinUpdateTitle(const char *title);
void DarwinPositionPopover(int width, int height);
void DarwinHidePopover(void);
void DarwinTogglePopover(int width, int height);

#ifdef __cplusplus
}
#endif

#endif // MANAGER_DARWIN_H

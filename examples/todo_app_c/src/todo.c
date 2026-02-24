// from vailang compiler
#include <stdio.h>
#include <string.h>


#define MAX_TODOS 100

#define TITLE_LEN 256

typedef struct {
    int id;
    char title[TITLE_LEN];
    int completed;
} Todo;

void print_menu(void) {
    printf("\n");
    printf("Todo Menu:\n");
    printf("1) Add\n");
    printf("2) List\n");
    printf("3) Complete\n");
    printf("4) Delete\n");
    printf("5) Quit\n");
}

void add_todo(Todo *todos, int *count) {
    if (*count >= MAX_TODOS) {
        fprintf(stderr, "Error: todo list is full.\n");
        return;
    }
    
    printf("Enter title: ");
    char buf[TITLE_LEN];
    fgets(buf, TITLE_LEN, stdin);
    
    char *newline = strchr(buf, '\n');
    if (newline) {
        *newline = '\0';
    }
    
    strncpy(todos[*count].title, buf, TITLE_LEN - 1);
    todos[*count].title[TITLE_LEN - 1] = '\0';
    
    todos[*count].id = *count + 1;
    todos[*count].completed = 0;
    
    int id = todos[*count].id;
    (*count)++;
    
    printf("Added todo #%d.\n", id);
}

void list_todos(const Todo *todos, int count) {
    if (count == 0) {
        printf("No todos.\n");
        return;
    }
    for (int i = 0; i < count; i++) {
        printf("[%d] %s (done: %s)\n", todos[i].id, todos[i].title, (todos[i].completed ? "yes" : "no"));
    }
}

void complete_todo(Todo *todos, int count) {
    printf("Enter id to mark complete: ");
    int id;
    scanf("%d", &id);
    
    int c;
    while ((c = getchar()) != '\n' && c != EOF) {}
    
    int found = 0;
    for (int i = 0; i < count; i++) {
        if (todos[i].id == id) {
            found = 1;
            if (todos[i].completed == 1) {
                printf("Todo #%d is already completed.\n", id);
            } else {
                todos[i].completed = 1;
                printf("Marked todo #%d as complete.\n", id);
            }
            break;
        }
    }
    
    if (!found) {
        fprintf(stderr, "Error: todo #%d not found.\n", id);
    }
}

void delete_todo(Todo *todos, int *count) {
    printf("Enter id to delete: ");
    int id;
    scanf("%d", &id);
    
    int c;
    while ((c = getchar()) != '\n' && c != EOF);
    
    int i;
    for (i = 0; i < *count; i++) {
        if (todos[i].id == id) {
            break;
        }
    }
    
    if (i == *count) {
        fprintf(stderr, "Error: todo #%d not found.\n", id);
        return;
    }
    
    memmove(&todos[i], &todos[i+1], (*count - i - 1) * sizeof(Todo));
    (*count)--;
    printf("Deleted todo #%d.\n", id);
}

int main(void) {
    Todo todos[MAX_TODOS];
    int count = 0;
    int choice;
    
    for(;;) {
        print_menu();
        scanf("%d", &choice);
        int c;
        while((c = getchar()) != '\n' && c != EOF);
        
        switch(choice) {
            case 1:
                add_todo(todos, &count);
                break;
            case 2:
                list_todos(todos, count);
                break;
            case 3:
                complete_todo(todos, count);
                break;
            case 4:
                delete_todo(todos, &count);
                break;
            case 5:
                printf("Goodbye.\n");
                return 0;
            default:
                fprintf(stderr, "Invalid choice.\n");
                break;
        }
    }
    
    return 0;
}

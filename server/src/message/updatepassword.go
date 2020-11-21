package message

type UpdatePasswordData struct {
    Password string
}

func NewUpdatePassword(password string) Client {
    return Client {
        CType: UpdatePassword,
        Data: UpdatePasswordData{
            Password: password,
        },
    }
}

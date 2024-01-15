import torch
from torch import nn
from d2l import torch as d2l
from sklearn.model_selection import KFold
from matplotlib import pyplot as plt
from matplotlib.gridspec import GridSpec
import os 
import warnings
warnings.filterwarnings('ignore')

DATA_PATH="./data"

def save(data, path):
    try:
        os.remove(path)
    except FileNotFoundError:
        pass
    except:
        raise
    with open(f"{DATA_PATH}/{path}", "wb+") as f:
        torch.save(data, f)

def load():
    train_features, test_features, train_labels = None, None, None
    with open(f"{DATA_PATH}/train.bin", "rb") as f:
        train_features = torch.load(f)
    with open(f"{DATA_PATH}/test.bin", mode="rb") as f:
        test_features = torch.load(f)
    with open(f"{DATA_PATH}/label.bin", "rb") as f:
        train_labels = torch.load(f)
    return train_features, test_features, train_labels

class Net(nn.Module):
    def __init__(self, num_inputs, num_outputs, hidden_layers=None, 
                 dropouts=None, active_func=nn.ReLU()):
        if dropouts and len(dropouts) != len(hidden_layers):
            raise ValueError("length of dropouts not equal to number of hidden layer")
        super().__init__()
        print(f"init net: {num_inputs}, {num_outputs}, {hidden_layers}")
        self.active_func = active_func
        self.hidden_layer_modules = []
        self.dropout_modules = []
        self.num_inputs = num_inputs
        self.num_outputs = num_outputs
        if hidden_layers:
            for i in hidden_layers:
                self.hidden_layer_modules.append(nn.Linear(num_inputs, i))
                num_inputs = i
        if dropouts:
            for p in dropouts:
                self.dropout_modules.append(nn.Dropout(p))

        self.output_layer = nn.Linear(num_inputs, num_outputs)

    def forward(self, X):
        h = X.reshape((-1, self.num_inputs))
        for i, module in enumerate(self.hidden_layer_modules):
            h = module(h)
            if self.dropout_modules and self.training:
                h = self.dropout_modules[i](h)
            self.active_func(h)
        out = self.output_layer(h)
        return out
    
def train(net, train_features, train_labels, test_features, test_labels,
          num_epochs, learning_rate, weight_decay, batch_size, loss, 
          need_plot=True, animator=None):
    if isinstance(net, nn.Module):
        net.train()
    train_ls, test_ls = [], []
    train_iter = d2l.load_array((train_features, train_labels), batch_size)
    optimizer = torch.optim.Adam(net.parameters(),
                                 lr = learning_rate,
                                 weight_decay = weight_decay)
    if need_plot:
        if not animator:
            legend = ["train loss"]
            if test_labels:
                legend.append("test loss")
            animator = d2l.Animator(xlabel="epoch", ylabel="loss", 
                                    xlim=[1, num_epochs+1], yscale="log", 
                                    legend=legend, figsize=(6, 4))
    for epoch in range(num_epochs):
        for X, y in train_iter:
            optimizer.zero_grad()
            l = loss(net(X), y)
            l.backward()
            optimizer.step()
        with torch.no_grad():
            l = loss(net(train_features), train_labels)
            train_ls.append(l)
        loss_l = [l]
        if test_labels is not None:
            with torch.no_grad():
                test_l = loss(net(test_features), test_labels)
                test_ls.append(test_l)
            loss_l.append(test_l)
        if need_plot:
            animator.add(1+epoch, loss_l)
    return train_ls, test_ls

d2l.plot

def k_fold_train(get_net, X_train, y_train,
          num_epochs, learning_rate, weight_decay, batch_size, loss, need_plot=True, k=5):
    kf = KFold(k, shuffle=True)
    ncols = 3
    nrows = (k + ncols) // ncols 
    if need_plot:
        fig = plt.figure(layout="constrained", figsize=(6*ncols, 4*nrows))
        gs = GridSpec(nrows, ncols, fig)
        i, j = 0, 0 
    train_loss_sum, valid_loss_sum = 0, 0
    for t, (train_index, valid_index) in enumerate(kf.split(X_train)):
        train_features, valid_features = X_train[train_index], X_train[valid_index]
        train_labels, valid_labels = y_train[train_index], y_train[valid_index]
        if need_plot:
            ax = fig.add_subplot(gs[i, j])
            j += 1
            if j == ncols:
                i += 1
                j = 0
            ax.set_xlim(1, num_epochs)
            ax.set_xlabel("epoch"), ax.set_ylabel("rmse loss")
            ax.set_title(r"k={}".format(t+1))
        train_loss, valid_loss = train(get_net(), train_features, train_labels, valid_features, valid_labels,
                                       num_epochs, learning_rate, weight_decay, batch_size, loss,
                                       need_plot=False)
        train_loss_sum += train_loss[-1]
        valid_loss_sum += valid_loss[-1]
        if need_plot:
            for label, curr_loss in zip(("train", "valid"), (train_loss, valid_loss)):
                ax.plot(list(range(1, num_epochs+1)), curr_loss, label=label)
            ax.legend() 
    return train_loss_sum / k, valid_loss_sum / k

train_features, test_features, train_labels = load()
num_inputs, num_outputs = train_features.shape[-1], 1
assert train_features.dtype == torch.float32 and test_features.dtype == torch.float32 and train_labels.dtype == torch.float32, \
    f"{train_features.dtype} {test_features.dtype}, {train_labels.dtype}"
print("load successful")

hidden_layers = []
droupouts = []
num_epochs = 100
weight_decay = 0
learning_rate = 0.1
batch_size=256
loss = nn.MSELoss()
k = 5

print("start train")
timer = d2l.Timer()
model1 = Net(num_inputs, num_outputs)
train_loss, _ = train(model1, train_features, train_labels, None, None, num_epochs, learning_rate, 
      weight_decay, batch_size, loss, need_plot=False)
print("finish train, elapsed sec: ", timer.stop())
train_loss = torch.tensor(train_loss, dtype=torch.float32)
print("start save loss")
save(train_loss, "loss.bin")
print("exit~~~")